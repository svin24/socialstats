package oauth

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/oauth2"
)

func dummyToken() *oauth2.Token {
	return &oauth2.Token{RefreshToken: "refresh-example", TokenType: "Bearer"}
}

func TestLoadConfigDesktopJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "client_secret.json")
	raw := `{
	  "installed": {
	    "client_id": "example.apps.googleusercontent.com",
	    "project_id": "example",
	    "auth_uri": "https://accounts.google.com/o/oauth2/auth",
	    "token_uri": "https://oauth2.googleapis.com/token",
	    "client_secret": "not-a-real-secret",
	    "redirect_uris": ["http://localhost"]
	  }
	}`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ClientID != "example.apps.googleusercontent.com" {
		t.Fatalf("client id %s", cfg.ClientID)
	}
	if len(cfg.Scopes) != 2 {
		t.Fatalf("scopes %v", cfg.Scopes)
	}
}

func TestLoadConfigMissing(t *testing.T) {
	_, err := LoadConfig(filepath.Join(t.TempDir(), "missing.json"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSaveLoadTokenRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token.json")
	in := dummyToken()
	if err := SaveToken(path, in); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("perm %o", st.Mode().Perm())
	}
	out, err := LoadToken(path)
	if err != nil {
		t.Fatal(err)
	}
	if out.RefreshToken != in.RefreshToken {
		t.Fatalf("got %s", out.RefreshToken)
	}
}
