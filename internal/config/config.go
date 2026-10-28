package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"socialstats/internal/paths"
)

// Version is stamped by the Nix build via -X when set.
var Version = "0.1.0"

// File is the optional JSON document in the config directory.
// YouTubeChannelID and XHandle have no built-in default: an empty channel id
// is detected from the authorized account, and the X handle is only a label.
type File struct {
	YouTubeChannelID string `json:"youtubeChannelId,omitempty"`
	XHandle          string `json:"xHandle,omitempty"`
	XWatchDir        string `json:"xWatchDir,omitempty"`
	WebhookURL       string `json:"webhookUrl,omitempty"`
	WebhookURLFile   string `json:"webhookUrlFile,omitempty"`
	ClientSecret     string `json:"clientSecret,omitempty"`
}

// Config is the resolved runtime configuration. Secrets live as files under
// ConfigDir, never in the repository.
type Config struct {
	ConfigDir        string
	DataDir          string
	YouTubeChannelID string
	XHandle          string
	XWatchDir        string
	WebhookURL       string
	WebhookURLFile   string
	ClientSecretPath string
	TokenPath        string
	StatePath        string
	ConfigFilePath   string
	DryRun           bool
}

type Overrides struct {
	ConfigDir        string
	DataDir          string
	YouTubeChannelID string
	XHandle          string
	XWatchDir        string
	WebhookURL       string
	WebhookURLFile   string
	ClientSecretPath string
	DryRun           bool
}

func Load(ov Overrides) (Config, error) {
	cfgDir := first(
		ov.ConfigDir,
		os.Getenv("SOCIALSTATS_CONFIG_DIR"),
		paths.ConfigDir(),
	)
	cfgDir = paths.ExpandUser(cfgDir)

	dataDir := first(
		ov.DataDir,
		os.Getenv("SOCIALSTATS_DATA_DIR"),
		paths.DataDir(),
	)
	dataDir = paths.ExpandUser(dataDir)

	file, err := readFile(filepath.Join(cfgDir, "config.json"))
	if err != nil {
		return Config{}, err
	}

	c := Config{
		ConfigDir:        cfgDir,
		DataDir:          dataDir,
		XWatchDir:        filepath.Join(home(), "Downloads"),
		ClientSecretPath: filepath.Join(cfgDir, "client_secret.json"),
		TokenPath:        filepath.Join(cfgDir, "token.json"),
		StatePath:        filepath.Join(cfgDir, "state.json"),
		ConfigFilePath:   filepath.Join(cfgDir, "config.json"),
		DryRun:           ov.DryRun,
	}

	if file.YouTubeChannelID != "" {
		c.YouTubeChannelID = file.YouTubeChannelID
	}
	if file.XHandle != "" {
		c.XHandle = strings.TrimPrefix(file.XHandle, "@")
	}
	if file.XWatchDir != "" {
		c.XWatchDir = paths.ExpandUser(file.XWatchDir)
	}
	if file.WebhookURL != "" {
		c.WebhookURL = file.WebhookURL
	}
	if file.WebhookURLFile != "" {
		c.WebhookURLFile = paths.ExpandUser(file.WebhookURLFile)
	}
	if file.ClientSecret != "" {
		c.ClientSecretPath = paths.ExpandUser(file.ClientSecret)
	}

	if v := os.Getenv("SOCIALSTATS_CHANNEL_ID"); v != "" {
		c.YouTubeChannelID = v
	}
	if v := os.Getenv("SOCIALSTATS_X_HANDLE"); v != "" {
		c.XHandle = strings.TrimPrefix(v, "@")
	}
	if v := os.Getenv("SOCIALSTATS_X_WATCH_DIR"); v != "" {
		c.XWatchDir = paths.ExpandUser(v)
	}
	if v := os.Getenv("SOCIALSTATS_WEBHOOK_URL"); v != "" {
		c.WebhookURL = v
	}
	if v := os.Getenv("SOCIALSTATS_WEBHOOK_URL_FILE"); v != "" {
		c.WebhookURLFile = paths.ExpandUser(v)
	}
	if v := os.Getenv("SOCIALSTATS_CLIENT_SECRET"); v != "" {
		c.ClientSecretPath = paths.ExpandUser(v)
	}

	if ov.YouTubeChannelID != "" {
		c.YouTubeChannelID = ov.YouTubeChannelID
	}
	if ov.XHandle != "" {
		c.XHandle = strings.TrimPrefix(ov.XHandle, "@")
	}
	if ov.XWatchDir != "" {
		c.XWatchDir = paths.ExpandUser(ov.XWatchDir)
	}
	if ov.WebhookURL != "" {
		c.WebhookURL = ov.WebhookURL
	}
	if ov.WebhookURLFile != "" {
		c.WebhookURLFile = paths.ExpandUser(ov.WebhookURLFile)
	}
	if ov.ClientSecretPath != "" {
		c.ClientSecretPath = paths.ExpandUser(ov.ClientSecretPath)
	}

	if c.WebhookURL == "" && c.WebhookURLFile != "" {
		url, err := readWebhookFile(c.WebhookURLFile)
		if err != nil {
			return Config{}, err
		}
		c.WebhookURL = url
	}

	c.XHandle = strings.TrimPrefix(c.XHandle, "@")
	return c, nil
}

// WriteOverlay merges non-empty fields into the JSON config file.
// Empty overlay fields are left as they already are. A zero overlay writes nothing.
func WriteOverlay(path string, overlay File) error {
	if overlay == (File{}) {
		return nil
	}
	existing, err := readFile(path)
	if err != nil {
		return err
	}
	if v := strings.TrimSpace(overlay.YouTubeChannelID); v != "" {
		existing.YouTubeChannelID = v
	}
	if v := strings.TrimSpace(overlay.XHandle); v != "" {
		existing.XHandle = strings.TrimPrefix(v, "@")
	}
	if v := strings.TrimSpace(overlay.XWatchDir); v != "" {
		existing.XWatchDir = v
	}
	if v := strings.TrimSpace(overlay.WebhookURL); v != "" {
		existing.WebhookURL = v
	}
	if v := strings.TrimSpace(overlay.WebhookURLFile); v != "" {
		existing.WebhookURLFile = v
	}
	if v := strings.TrimSpace(overlay.ClientSecret); v != "" {
		existing.ClientSecret = v
	}
	b, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("config dir: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func (c Config) EnsureDirs() error {
	if err := os.MkdirAll(c.ConfigDir, 0o700); err != nil {
		return fmt.Errorf("config dir: %w", err)
	}
	if err := os.MkdirAll(c.DataDir, 0o755); err != nil {
		return fmt.Errorf("data dir: %w", err)
	}
	return nil
}

func (c Config) HasClientSecret() bool {
	return fileExists(c.ClientSecretPath)
}

func (c Config) HasToken() bool {
	return fileExists(c.TokenPath)
}

func readFile(path string) (File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return File{}, nil
		}
		return File{}, fmt.Errorf("read config: %w", err)
	}
	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		return File{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return f, nil
}

func readWebhookFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("webhook url file: %w", err)
	}
	s := strings.TrimSpace(string(b))
	s = strings.TrimPrefix(s, "SOCIALSTATS_WEBHOOK_URL=")
	s = strings.Trim(s, `"'`)
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("webhook url file %s is empty", path)
	}
	return s, nil
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func first(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func home() string {
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return h
	}
	return os.Getenv("HOME")
}
