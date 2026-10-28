package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigAndDataDirXDG(t *testing.T) {
	t.Setenv("SOCIALSTATS_CONFIG_DIR", "")
	t.Setenv("SOCIALSTATS_DATA_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-config")
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg-data")
	if got := ConfigDir(); got != filepath.Join("/tmp/xdg-config", "socialstats") {
		t.Fatalf("ConfigDir=%s", got)
	}
	if got := DataDir(); got != filepath.Join("/tmp/xdg-data", "socialstats") {
		t.Fatalf("DataDir=%s", got)
	}
}

func TestExplicitEnvWins(t *testing.T) {
	t.Setenv("SOCIALSTATS_CONFIG_DIR", "/tmp/ss-cfg")
	t.Setenv("SOCIALSTATS_DATA_DIR", "/tmp/ss-data")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-config")
	if ConfigDir() != "/tmp/ss-cfg" {
		t.Fatalf("config %s", ConfigDir())
	}
	if DataDir() != "/tmp/ss-data" {
		t.Fatalf("data %s", DataDir())
	}
}

func TestExpandUser(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	got := ExpandUser("~/Downloads")
	want := filepath.Join(home, "Downloads")
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}
