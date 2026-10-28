package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaultsAndOverrides(t *testing.T) {
	dir := t.TempDir()
	cfgJSON := `{"youtubeChannelId":"UCfromfile","xHandle":"@fromfile","webhookUrl":"https://example.test/fromfile"}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(cfgJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SOCIALSTATS_CONFIG_DIR", dir)
	t.Setenv("SOCIALSTATS_DATA_DIR", filepath.Join(dir, "data"))
	t.Setenv("SOCIALSTATS_WEBHOOK_URL", "")
	t.Setenv("SOCIALSTATS_CHANNEL_ID", "")
	t.Setenv("SOCIALSTATS_X_HANDLE", "")

	c, err := Load(Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if c.YouTubeChannelID != "UCfromfile" {
		t.Fatalf("channel %s", c.YouTubeChannelID)
	}
	if c.XHandle != "fromfile" {
		t.Fatalf("handle %s", c.XHandle)
	}
	if c.WebhookURL != "https://example.test/fromfile" {
		t.Fatalf("webhook %s", c.WebhookURL)
	}
	if c.TokenPath != filepath.Join(dir, "token.json") {
		t.Fatalf("token path %s", c.TokenPath)
	}

	c, err = Load(Overrides{YouTubeChannelID: "UCflag", WebhookURL: "https://example.test/flag"})
	if err != nil {
		t.Fatal(err)
	}
	if c.YouTubeChannelID != "UCflag" || c.WebhookURL != "https://example.test/flag" {
		t.Fatalf("overrides not applied: %+v", c)
	}
}

func TestWebhookURLFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hook")
	if err := os.WriteFile(path, []byte("https://hooks.example.test/abc\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SOCIALSTATS_CONFIG_DIR", dir)
	t.Setenv("SOCIALSTATS_DATA_DIR", dir)
	t.Setenv("SOCIALSTATS_WEBHOOK_URL", "")
	c, err := Load(Overrides{WebhookURLFile: path})
	if err != nil {
		t.Fatal(err)
	}
	if c.WebhookURL != "https://hooks.example.test/abc" {
		t.Fatalf("got %q", c.WebhookURL)
	}
}

func TestEmptyDefaultsThenEnvThenFlag(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SOCIALSTATS_CONFIG_DIR", dir)
	t.Setenv("SOCIALSTATS_DATA_DIR", dir)
	t.Setenv("SOCIALSTATS_CHANNEL_ID", "")
	t.Setenv("SOCIALSTATS_X_HANDLE", "")
	t.Setenv("SOCIALSTATS_WEBHOOK_URL", "")
	c, err := Load(Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if c.YouTubeChannelID != "" || c.XHandle != "" {
		t.Fatalf("expected empty defaults, got channel=%q handle=%q", c.YouTubeChannelID, c.XHandle)
	}

	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"youtubeChannelId":"UCfromfile00000000000000","xHandle":"fromfile"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err = Load(Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if c.YouTubeChannelID != "UCfromfile00000000000000" || c.XHandle != "fromfile" {
		t.Fatalf("file override: %+v", c)
	}

	t.Setenv("SOCIALSTATS_CHANNEL_ID", "UCfromenv000000000000000")
	t.Setenv("SOCIALSTATS_X_HANDLE", "@fromenv")
	c, err = Load(Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if c.YouTubeChannelID != "UCfromenv000000000000000" || c.XHandle != "fromenv" {
		t.Fatalf("env override: %+v", c)
	}

	c, err = Load(Overrides{YouTubeChannelID: "UCfromflag00000000000000", XHandle: "fromflag"})
	if err != nil {
		t.Fatal(err)
	}
	if c.YouTubeChannelID != "UCfromflag00000000000000" || c.XHandle != "fromflag" {
		t.Fatalf("flag override: %+v", c)
	}
}

func TestWriteOverlayPreservesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("{\"webhookUrl\":\"https://example.test/hook\",\"xHandle\":\"kept\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteOverlay(path, File{}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "{\"webhookUrl\":\"https://example.test/hook\",\"xHandle\":\"kept\"}\n" {
		t.Fatalf("empty overlay changed file: %s", b)
	}
	if err := WriteOverlay(path, File{YouTubeChannelID: "UCexample000000000000000", XHandle: "@example_handle"}); err != nil {
		t.Fatal(err)
	}
	c, err := readFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.YouTubeChannelID != "UCexample000000000000000" || c.XHandle != "example_handle" || c.WebhookURL != "https://example.test/hook" {
		t.Fatalf("merged %+v", c)
	}
}
