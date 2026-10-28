package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"socialstats/internal/store"
)

const userAgent = "socialstats/0.1.0"

type Result struct {
	URL        string `json:"url"`
	StatusCode int    `json:"statusCode"`
	Skipped    bool   `json:"skipped,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

func Post(ctx context.Context, rawURL string, latest store.Latest, dryRun bool) (Result, error) {
	if strings.TrimSpace(rawURL) == "" {
		return Result{Skipped: true, Reason: "no webhook URL configured"}, nil
	}
	if dryRun {
		return Result{URL: redact(rawURL), Skipped: true, Reason: "dry-run"}, nil
	}

	body, err := json.Marshal(latest)
	if err != nil {
		return Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return Result{URL: redact(rawURL)}, fmt.Errorf("webhook: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return Result{URL: redact(rawURL), StatusCode: resp.StatusCode}, fmt.Errorf("webhook HTTP %d", resp.StatusCode)
	}
	return Result{URL: redact(rawURL), StatusCode: resp.StatusCode}, nil
}

func redact(rawURL string) string {
	// Never print query strings or userinfo (tokens often live there).
	if i := strings.Index(rawURL, "://"); i >= 0 {
		rest := rawURL[i+3:]
		host := rest
		if j := strings.IndexAny(rest, "/?"); j >= 0 {
			host = rest[:j]
		}
		if at := strings.LastIndex(host, "@"); at >= 0 {
			host = host[at+1:]
		}
		scheme := rawURL[:i]
		return scheme + "://" + host + "/…"
	}
	return "(webhook)"
}

// SecretFields is used by tests to assert payloads never leak credentials.
func SecretFields() []string {
	return []string{
		"refresh_token", "access_token", "client_secret", "clientSecret",
		"token.json", "client_secret.json", "Authorization",
	}
}
