package paths

import (
	"os"
	"path/filepath"
)

const AppName = "socialstats"

// ConfigDir is $SOCIALSTATS_CONFIG_DIR, else $XDG_CONFIG_HOME/socialstats,
// else ~/.config/socialstats.
func ConfigDir() string {
	if v := os.Getenv("SOCIALSTATS_CONFIG_DIR"); v != "" {
		return expand(v)
	}
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return filepath.Join(v, AppName)
	}
	return filepath.Join(home(), ".config", AppName)
}

// DataDir is $SOCIALSTATS_DATA_DIR, else $XDG_DATA_HOME/socialstats,
// else ~/.local/share/socialstats.
func DataDir() string {
	if v := os.Getenv("SOCIALSTATS_DATA_DIR"); v != "" {
		return expand(v)
	}
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return filepath.Join(v, AppName)
	}
	return filepath.Join(home(), ".local", "share", AppName)
}

func home() string {
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return h
	}
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return "."
}

func expand(p string) string {
	if p == "" {
		return p
	}
	if p == "~" || (len(p) >= 2 && p[0] == '~' && (p[1] == '/' || p[1] == filepath.Separator)) {
		return filepath.Join(home(), p[1:])
	}
	return p
}

// ExpandUser expands a leading ~ to the home directory.
func ExpandUser(p string) string { return expand(p) }
