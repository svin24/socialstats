package oauth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	ScopeYTAnalytics = "https://www.googleapis.com/auth/yt-analytics.readonly"
	ScopeYouTube     = "https://www.googleapis.com/auth/youtube.readonly"
)

// Scopes are read-only. yt-analytics-monetary.readonly is intentionally omitted.
var Scopes = []string{ScopeYTAnalytics, ScopeYouTube}

func LoadConfig(clientSecretPath string) (*oauth2.Config, error) {
	b, err := os.ReadFile(clientSecretPath)
	if err != nil {
		return nil, fmt.Errorf("read OAuth client file %s: %w\nCreate a Desktop OAuth client in Google Cloud and save the JSON here. See the README.", clientSecretPath, err)
	}
	cfg, err := google.ConfigFromJSON(b, Scopes...)
	if err != nil {
		return nil, fmt.Errorf("parse OAuth client JSON: %w (expected an installed-app / Desktop client)", err)
	}
	return cfg, nil
}

func LoadToken(path string) (*oauth2.Token, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read token %s: %w (run: socialstats auth)", path, err)
	}
	var tok oauth2.Token
	if err := json.Unmarshal(b, &tok); err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}
	if tok.RefreshToken == "" && !tok.Valid() {
		return nil, fmt.Errorf("token at %s has no refresh token and is expired; run: socialstats auth", path)
	}
	return &tok, nil
}

func SaveToken(path string, tok *oauth2.Token) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(tok, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// HTTPClient returns an OAuth2 HTTP client that refreshes using the stored token.
func HTTPClient(ctx context.Context, clientSecretPath, tokenPath string) (*http.Client, error) {
	cfg, err := LoadConfig(clientSecretPath)
	if err != nil {
		return nil, err
	}
	tok, err := LoadToken(tokenPath)
	if err != nil {
		return nil, err
	}
	return cfg.Client(ctx, tok), nil
}

type AuthOptions struct {
	OpenBrowser bool
	Timeout     time.Duration
	ListenAddr  string // default 127.0.0.1:0
	Printf      func(string, ...any)
}

// Login runs the installed-app loopback flow and writes a refresh token.
func Login(ctx context.Context, clientSecretPath, tokenPath string, opt AuthOptions) error {
	if opt.Printf == nil {
		opt.Printf = func(string, ...any) {}
	}
	if opt.Timeout == 0 {
		opt.Timeout = 5 * time.Minute
	}
	if opt.ListenAddr == "" {
		opt.ListenAddr = "127.0.0.1:0"
	}

	cfg, err := LoadConfig(clientSecretPath)
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", opt.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen for OAuth loopback: %w", err)
	}
	defer ln.Close()

	addr := ln.Addr().(*net.TCPAddr)
	redirect := fmt.Sprintf("http://127.0.0.1:%d/callback", addr.Port)
	cfg.RedirectURL = redirect

	state, err := randomState()
	if err != nil {
		return err
	}

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			select {
			case errCh <- fmt.Errorf("oauth state mismatch"):
			default:
			}
			return
		}
		if e := r.URL.Query().Get("error"); e != "" {
			msg := e
			if d := r.URL.Query().Get("error_description"); d != "" {
				msg = msg + ": " + d
			}
			http.Error(w, msg, http.StatusBadRequest)
			select {
			case errCh <- fmt.Errorf("oauth: %s", msg):
			default:
			}
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			select {
			case errCh <- fmt.Errorf("oauth: missing code"):
			default:
			}
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!doctype html><p>socialstats is authorized. You can close this tab.</p>"))
		select {
		case codeCh <- code:
		default:
		}
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/callback?"+r.URL.RawQuery, http.StatusFound)
	})

	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer func() {
		cctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(cctx)
	}()

	authURL := cfg.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
	)
	opt.Printf("Open this URL in a browser and authorize the read-only YouTube scopes:\n\n%s\n", authURL)
	if opt.OpenBrowser {
		if err := openBrowser(authURL); err != nil {
			opt.Printf("Could not open a browser automatically: %v\n", err)
		}
	}

	ctx, cancel := context.WithTimeout(ctx, opt.Timeout)
	defer cancel()

	var code string
	select {
	case <-ctx.Done():
		return fmt.Errorf("oauth timed out waiting for the browser callback")
	case err := <-errCh:
		return err
	case code = <-codeCh:
	}

	tok, err := cfg.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("token exchange: %w", err)
	}
	if tok.RefreshToken == "" {
		return fmt.Errorf("Google did not return a refresh token; delete the app from https://myaccount.google.com/permissions and run socialstats auth again")
	}
	if err := SaveToken(tokenPath, tok); err != nil {
		return err
	}
	opt.Printf("Saved refresh token to %s (mode 0600).\n", tokenPath)
	return nil
}

func randomState() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func openBrowser(url string) error {
	return exec.Command("xdg-open", url).Start()
}
