package webhook

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"socialstats/internal/store"
)

func TestPostDryRunDoesNotHitServer(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(200)
	}))
	defer srv.Close()
	res, err := Post(context.Background(), srv.URL, store.Latest{GeneratedAt: "x"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Skipped || hits != 0 {
		t.Fatalf("skipped=%v hits=%d", res.Skipped, hits)
	}
}

func TestPostJSONAndNoSecrets(t *testing.T) {
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content-type %s", r.Header.Get("Content-Type"))
		}
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(204)
	}))
	defer srv.Close()
	latest := store.Latest{
		SchemaVersion: 1,
		GeneratedAt:   "2026-10-28T00:00:00Z",
		YouTube: &store.YouTubeLatest{
			ChannelID: "UC123",
			Totals:    map[string]any{"views": 10},
		},
	}
	res, err := Post(context.Background(), srv.URL+"?token=supersecret", latest, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 204 {
		t.Fatalf("status %d", res.StatusCode)
	}
	if strings.Contains(res.URL, "supersecret") {
		t.Fatalf("url not redacted: %s", res.URL)
	}
	raw := string(body)
	for _, s := range SecretFields() {
		if strings.Contains(raw, s) {
			t.Fatalf("payload leaked %s", s)
		}
	}
	var decoded store.Latest
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.YouTube.ChannelID != "UC123" {
		t.Fatalf("payload %+v", decoded)
	}
}

func TestEmptyURLSkipped(t *testing.T) {
	res, err := Post(context.Background(), "", store.Latest{}, false)
	if err != nil || !res.Skipped {
		t.Fatalf("%v %+v", err, res)
	}
}

func TestHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	_, err := Post(context.Background(), srv.URL, store.Latest{}, false)
	if err == nil {
		t.Fatal("expected error")
	}
}
