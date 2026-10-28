package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"socialstats/internal/xcsv"
)

func TestHelp(t *testing.T) {
	var out, err bytes.Buffer
	code := MainIO([]string{"socialstats", "--help"}, &out, &err)
	if code != 0 {
		t.Fatalf("code %d err=%s", code, err.String())
	}
	s := out.String()
	for _, want := range []string{"auth", "fetch", "import-x", "status", "setup", "--dry-run", "YouTube"} {
		if !strings.Contains(s, want) {
			t.Fatalf("help missing %q in %s", want, s)
		}
	}
}

func TestVersion(t *testing.T) {
	var out, err bytes.Buffer
	code := MainIO([]string{"socialstats", "version"}, &out, &err)
	if code != 0 || !strings.Contains(out.String(), ".") {
		t.Fatalf("code %d out=%s err=%s", code, out.String(), err.String())
	}
}

func TestUnknownCommand(t *testing.T) {
	var out, errb bytes.Buffer
	code := MainIO([]string{"socialstats", "explode"}, &out, &errb)
	if code != 2 {
		t.Fatalf("code %d", code)
	}
}

func TestFetchDryRunNoCredentials(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SOCIALSTATS_CONFIG_DIR", dir)
	t.Setenv("SOCIALSTATS_DATA_DIR", filepath.Join(dir, "data"))
	t.Setenv("SOCIALSTATS_WEBHOOK_URL", "")
	var out, errb bytes.Buffer
	code := MainIO([]string{"socialstats", "--dry-run", "fetch", "--since", "2026-10-01", "--until", "2026-10-14"}, &out, &errb)
	if code != 0 {
		t.Fatalf("code %d err=%s out=%s", code, errb.String(), out.String())
	}
	s := out.String()
	for _, want := range []string{"dry-run", "views", "insightTrafficSourceType", "2026-10-01", "2026-10-14"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in %s", want, s)
		}
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "data"))
	if len(entries) != 0 {
		t.Fatalf("dry-run wrote data: %v", entries)
	}
}

func TestAuthAndFetchPrintSetupWhenSecretMissing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SOCIALSTATS_CONFIG_DIR", dir)
	t.Setenv("SOCIALSTATS_DATA_DIR", filepath.Join(dir, "data"))
	t.Setenv("SOCIALSTATS_WEBHOOK_URL", "")
	secret := filepath.Join(dir, "client_secret.json")
	for _, args := range [][]string{
		{"socialstats", "auth"},
		{"socialstats", "fetch", "--since", "2026-10-01", "--until", "2026-10-02"},
	} {
		var out, errb bytes.Buffer
		code := MainIO(args, &out, &errb)
		if code != 1 {
			t.Fatalf("%v code %d out=%s err=%s", args, code, out.String(), errb.String())
		}
		guide := errb.String()
		for _, want := range []string{
			"https://console.cloud.google.com/projectcreate",
			"https://console.cloud.google.com/apis/library/youtubeanalytics.googleapis.com",
			"https://console.cloud.google.com/apis/library/youtube.googleapis.com",
			"https://console.cloud.google.com/auth/overview",
			"https://console.cloud.google.com/auth/clients",
			secret,
			"socialstats auth",
			"socialstats fetch",
			"7 days",
		} {
			if !strings.Contains(guide, want) {
				t.Fatalf("%v guide missing %q in %s", args, want, guide)
			}
		}
	}
}

func TestFetchDryRunNamesAutoDetect(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SOCIALSTATS_CONFIG_DIR", dir)
	t.Setenv("SOCIALSTATS_DATA_DIR", filepath.Join(dir, "data"))
	t.Setenv("SOCIALSTATS_CHANNEL_ID", "")
	t.Setenv("SOCIALSTATS_X_HANDLE", "")
	t.Setenv("SOCIALSTATS_WEBHOOK_URL", "")
	var out, errb bytes.Buffer
	code := MainIO([]string{"socialstats", "--dry-run", "fetch", "--since", "2026-10-01", "--until", "2026-10-02"}, &out, &errb)
	if code != 0 {
		t.Fatalf("code %d err=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "auto-detect (channels?mine=true)") {
		t.Fatalf("out %s", out.String())
	}
	if !strings.Contains(errb.String(), "https://console.cloud.google.com/projectcreate") {
		t.Fatalf("dry-run fetch should still print setup, err=%s", errb.String())
	}
}

func TestSetupWritesConfigFromFlags(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SOCIALSTATS_CONFIG_DIR", dir)
	t.Setenv("SOCIALSTATS_DATA_DIR", filepath.Join(dir, "data"))
	t.Setenv("SOCIALSTATS_WEBHOOK_URL", "")
	old := setupInput
	setupInput = strings.NewReader("")
	t.Cleanup(func() { setupInput = old })

	var out, errb bytes.Buffer
	code := MainIO([]string{"socialstats", "setup", "--channel-id", "UCexample000000000000000", "--x-handle", "@example_handle"}, &out, &errb)
	if code != 0 {
		t.Fatalf("code %d err=%s out=%s", code, errb.String(), out.String())
	}
	if !strings.Contains(out.String(), "https://console.cloud.google.com/auth/clients") {
		t.Fatalf("setup did not print the guide: %s", out.String())
	}
	b, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	if !strings.Contains(text, `"youtubeChannelId": "UCexample000000000000000"`) || !strings.Contains(text, `"xHandle": "example_handle"`) {
		t.Fatalf("config %s", text)
	}
	st, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o700 {
		t.Fatalf("config dir mode %o", st.Mode().Perm())
	}

	out.Reset()
	errb.Reset()
	code = MainIO([]string{"socialstats", "init"}, &out, &errb)
	if code != 0 {
		t.Fatalf("init code %d err=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "unchanged") {
		t.Fatalf("init rewrote config: %s", out.String())
	}
}

func TestAuthDryRun(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SOCIALSTATS_CONFIG_DIR", dir)
	t.Setenv("SOCIALSTATS_DATA_DIR", dir)
	var out, errb bytes.Buffer
	code := MainIO([]string{"socialstats", "auth", "--dry-run"}, &out, &errb)
	if code != 0 {
		t.Fatalf("code %d err=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "loopback") {
		t.Fatalf("out %s", out.String())
	}
}

func TestStatusJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SOCIALSTATS_CONFIG_DIR", dir)
	t.Setenv("SOCIALSTATS_DATA_DIR", dir)
	t.Setenv("SOCIALSTATS_WEBHOOK_URL", "")
	var out, errb bytes.Buffer
	code := MainIO([]string{"socialstats", "status", "--json"}, &out, &errb)
	if code != 0 {
		t.Fatalf("code %d err=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), `"hasToken": false`) {
		t.Fatalf("out %s", out.String())
	}
	if strings.Contains(out.String(), "refresh_token") {
		t.Fatal("status leaked token field")
	}
}

func TestImportXDryRun(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SOCIALSTATS_CONFIG_DIR", dir)
	t.Setenv("SOCIALSTATS_DATA_DIR", filepath.Join(dir, "data"))
	t.Setenv("SOCIALSTATS_WEBHOOK_URL", "")
	csv := filepath.Join("..", "..", "testdata", "x_post_activity.csv")
	var out, errb bytes.Buffer
	code := MainIO([]string{"socialstats", "import-x", "--dry-run", csv}, &out, &errb)
	if code != 0 {
		t.Fatalf("code %d err=%s out=%s", code, errb.String(), out.String())
	}
	if !strings.Contains(out.String(), "kind=posts") {
		t.Fatalf("out %s", out.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "data", "latest.json")); !os.IsNotExist(err) {
		t.Fatal("dry-run wrote latest.json")
	}
}

func TestImportXWritesDedupedLatest(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SOCIALSTATS_CONFIG_DIR", dir)
	t.Setenv("SOCIALSTATS_DATA_DIR", filepath.Join(dir, "data"))
	t.Setenv("SOCIALSTATS_WEBHOOK_URL", "")
	csv := filepath.Join("..", "..", "testdata", "x_account_analytics.csv")
	var out, errb bytes.Buffer
	code := MainIO([]string{"socialstats", "import-x", "--no-watch", csv}, &out, &errb)
	if code != 0 {
		t.Fatalf("code %d err=%s out=%s", code, errb.String(), out.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "data", "latest.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "data", "x", "daily.csv")); err != nil {
		t.Fatal(err)
	}
	// Second run should skip by hash.
	out.Reset()
	errb.Reset()
	code = MainIO([]string{"socialstats", "import-x", "--no-watch", csv}, &out, &errb)
	if code != 0 {
		t.Fatalf("second code %d err=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "already imported") && !strings.Contains(out.String(), "nothing new") {
		t.Fatalf("expected skip, got %s", out.String())
	}
}

func TestImportXAccountOverviewDedup(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SOCIALSTATS_CONFIG_DIR", dir)
	t.Setenv("SOCIALSTATS_DATA_DIR", filepath.Join(dir, "data"))
	t.Setenv("SOCIALSTATS_WEBHOOK_URL", "")
	csv := filepath.Join("..", "..", "testdata", "x_account_overview.csv")

	var out, errb bytes.Buffer
	code := MainIO([]string{"socialstats", "import-x", "--no-watch", csv}, &out, &errb)
	if code != 0 {
		t.Fatalf("code %d err=%s out=%s", code, errb.String(), out.String())
	}
	assertOverviewSummary(t, filepath.Join(dir, "data"))

	out.Reset()
	errb.Reset()
	code = MainIO([]string{"socialstats", "import-x", "--no-watch", csv}, &out, &errb)
	if code != 0 {
		t.Fatalf("second code %d err=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "already imported") && !strings.Contains(out.String(), "nothing new") {
		t.Fatalf("expected skip, got %s", out.String())
	}
	rows := readDailyRows(t, filepath.Join(dir, "data", "x", "daily.json"))
	if len(rows) != 3 {
		t.Fatalf("re-import duplicated rows: %d", len(rows))
	}
}

func TestImportXReprocessesLegacyOverviewOnce(t *testing.T) {
	dir := t.TempDir()
	data := filepath.Join(dir, "data")
	t.Setenv("SOCIALSTATS_CONFIG_DIR", dir)
	t.Setenv("SOCIALSTATS_DATA_DIR", data)
	t.Setenv("SOCIALSTATS_WEBHOOK_URL", "")
	csv := filepath.Join("..", "..", "testdata", "x_account_overview.csv")
	parsed, err := xcsv.ParseFile(csv)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(data, "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := []map[string]any{
		{"date": "Fri, Jul 31, 2026", "impressions": 1},
		{"date": "Wed, Sep 9, 2026", "impressions": 1},
		{"date": "Wed, Oct 28, 2026", "impressions": 1},
	}
	writeJSONFile(t, filepath.Join(data, "x", "daily.json"), legacy)
	// Version 0 matches files imported before weekday dates were parsed.
	st := map[string]any{
		"x": map[string]any{
			"files": map[string]any{
				parsed.SHA256: map[string]any{
					"path": csv, "sha256": parsed.SHA256, "importedAt": "2026-10-28T00:00:00Z", "rows": 3,
				},
			},
		},
	}
	writeJSONFile(t, filepath.Join(dir, "state.json"), st)

	var out, errb bytes.Buffer
	code := MainIO([]string{"socialstats", "import-x", "--no-watch", csv}, &out, &errb)
	if code != 0 {
		t.Fatalf("code %d err=%s out=%s", code, errb.String(), out.String())
	}
	if strings.Contains(out.String(), "already imported") {
		t.Fatalf("legacy import was skipped: %s", out.String())
	}
	assertOverviewSummary(t, data)
	rows := readDailyRows(t, filepath.Join(data, "x", "daily.json"))
	if len(rows) != 3 {
		t.Fatalf("legacy dates were not collapsed: %d %+v", len(rows), rows)
	}

	out.Reset()
	errb.Reset()
	code = MainIO([]string{"socialstats", "import-x", "--no-watch", csv}, &out, &errb)
	if code != 0 {
		t.Fatalf("second code %d err=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "already imported") {
		t.Fatalf("expected dedup, got %s", out.String())
	}
	rows = readDailyRows(t, filepath.Join(data, "x", "daily.json"))
	if len(rows) != 3 {
		t.Fatalf("second import duplicated rows: %d", len(rows))
	}
}

func assertOverviewSummary(t *testing.T, dataDir string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dataDir, "latest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	x, _ := doc["x"].(map[string]any)
	if x["rangeStart"] != "2026-07-31" || x["rangeEnd"] != "2026-10-28" {
		t.Fatalf("range %v .. %v", x["rangeStart"], x["rangeEnd"])
	}
	totals, _ := x["totals"].(map[string]any)
	want := map[string]float64{
		"impressions": 125, "engagements": 16, "likes": 13, "bookmarks": 3,
		"shares": 4, "newFollows": 5, "unfollows": 6, "replies": 7,
		"reposts": 8, "profileVisits": 9,
	}
	for k, v := range want {
		if totals[k] != v {
			t.Fatalf("totals[%s]=%#v want %v (all=%v)", k, totals[k], v, totals)
		}
	}
	csvRaw, err := os.ReadFile(filepath.Join(dataDir, "x", "daily.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(csvRaw), "2026-09-09") || strings.Contains(string(csvRaw), "Sep 9") {
		t.Fatalf("daily.csv dates not ISO:\n%s", csvRaw)
	}
}

func readDailyRows(t *testing.T, path string) []map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	return rows
}

func writeJSONFile(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFetchWithoutAuthFails(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SOCIALSTATS_CONFIG_DIR", dir)
	t.Setenv("SOCIALSTATS_DATA_DIR", dir)
	var out, errb bytes.Buffer
	code := MainIO([]string{"socialstats", "fetch", "--since", "2026-10-01", "--until", "2026-10-02"}, &out, &errb)
	if code == 0 {
		t.Fatal("expected failure without token")
	}
}
