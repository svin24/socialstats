package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"socialstats/internal/xcsv"
	"socialstats/internal/youtube"
)

func TestYouTubeDedupIncomingWins(t *testing.T) {
	dir := t.TempDir()
	s := Store{DataDir: dir}
	snap1 := youtube.Snapshot{
		FetchedAt:  "2026-10-27T00:00:00Z",
		RangeStart: "2026-10-01",
		RangeEnd:   "2026-10-27",
		Channel:    youtube.Channel{ID: "UCexample000000000000000", Title: "Example"},
		Daily: youtube.Report{Rows: []map[string]any{
			{"day": "2026-10-01", "views": 10.0},
			{"day": "2026-10-02", "views": 5.0},
		}},
		Videos: youtube.Report{Rows: []map[string]any{
			{"video": "aaa", "views": 3.0},
		}},
	}
	if err := s.WriteYouTube(snap1); err != nil {
		t.Fatal(err)
	}
	snap2 := snap1
	snap2.FetchedAt = "2026-10-28T00:00:00Z"
	snap2.Daily.Rows = []map[string]any{
		{"day": "2026-10-01", "views": 11.0},
		{"day": "2026-10-03", "views": 7.0},
	}
	if err := s.WriteYouTube(snap2); err != nil {
		t.Fatal(err)
	}
	rows, err := readMaps(filepath.Join(dir, "youtube", "daily.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows %d", len(rows))
	}
	by := map[string]float64{}
	for _, r := range rows {
		by[stringify(r["day"])] = num(r["views"])
	}
	if by["2026-10-01"] != 11 || by["2026-10-02"] != 5 || by["2026-10-03"] != 7 {
		t.Fatalf("merged %+v", by)
	}
	if _, err := os.Stat(filepath.Join(dir, "latest.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "youtube", "daily.csv")); err != nil {
		t.Fatal(err)
	}
}

func TestXPostDedupPrefersHigherImpressions(t *testing.T) {
	dir := t.TempDir()
	s := Store{DataDir: dir}
	imp := func(v float64) *float64 { return &v }
	first := []xcsv.Post{{ID: "1", Date: "2026-10-01", Impressions: imp(10)}}
	if err := s.WriteX("example_handle", first, nil); err != nil {
		t.Fatal(err)
	}
	second := []xcsv.Post{{ID: "1", Date: "2026-10-01", Impressions: imp(25)}, {ID: "2", Date: "2026-10-02", Impressions: imp(3)}}
	if err := s.WriteX("example_handle", second, nil); err != nil {
		t.Fatal(err)
	}
	posts, err := readPosts(filepath.Join(dir, "x", "posts.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 2 {
		t.Fatalf("len %d", len(posts))
	}
	if posts[0].Impressions == nil || *posts[0].Impressions != 25 {
		t.Fatalf("kept %v", posts[0].Impressions)
	}
}

func TestXOverviewSummaryRangeAndTotals(t *testing.T) {
	dir := t.TempDir()
	s := Store{DataDir: dir}
	res, err := xcsv.ParseFile(filepath.Join("..", "..", "testdata", "x_account_overview.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.WriteX("example_handle", nil, res.Daily); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "latest.json"))
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
		"impressions":   125,
		"engagements":   16,
		"likes":         13,
		"bookmarks":     3,
		"shares":        4,
		"newFollows":    5,
		"unfollows":     6,
		"replies":       7,
		"reposts":       8,
		"profileVisits": 9,
	}
	for k, v := range want {
		if totals[k] != v {
			t.Fatalf("totals[%s]=%#v want %v (all=%v)", k, totals[k], v, totals)
		}
	}
	dailyRaw, err := os.ReadFile(filepath.Join(dir, "x", "daily.csv"))
	if err != nil {
		t.Fatal(err)
	}
	dailyCSV := string(dailyRaw)
	for _, iso := range []string{"2026-07-31", "2026-09-09", "2026-10-28"} {
		if !contains(dailyCSV, iso) {
			t.Fatalf("daily.csv missing %s:\n%s", iso, dailyCSV)
		}
	}
	if contains(dailyCSV, "Wed,") || contains(dailyCSV, "Fri,") {
		t.Fatalf("daily.csv kept weekday dates:\n%s", dailyCSV)
	}
	latestCSV, err := os.ReadFile(filepath.Join(dir, "latest.csv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{
		"x,rangeStart,2026-07-31",
		"x,rangeEnd,2026-10-28",
		"x,impressions,125",
		"x,engagements,16",
		"x,likes,13",
		"x,bookmarks,3",
		"x,shares,4",
		"x,newFollows,5",
		"x,unfollows,6",
		"x,replies,7",
		"x,reposts,8",
		"x,profileVisits,9",
	} {
		if !contains(string(latestCSV), line) {
			t.Fatalf("latest.csv missing %q:\n%s", line, latestCSV)
		}
	}
}

func TestMergeDailyNormalizesLegacyWeekdayDates(t *testing.T) {
	dir := t.TempDir()
	s := Store{DataDir: dir}
	legacy := []xcsv.Daily{
		{Date: "Fri, Jul 31, 2026", Impressions: f64(1)},
		{Date: "Wed, Oct 28, 2026", Impressions: f64(2)},
	}
	if err := writeJSON(filepath.Join(dir, "x", "daily.json"), legacy); err != nil {
		t.Fatal(err)
	}
	incoming := []xcsv.Daily{{Date: "2026-10-28", Impressions: f64(100), Likes: f64(10)}}
	if err := s.WriteX("example_handle", nil, incoming); err != nil {
		t.Fatal(err)
	}
	rows, err := readDaily(filepath.Join(dir, "x", "daily.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows %d: %+v", len(rows), rows)
	}
	by := map[string]xcsv.Daily{}
	for _, r := range rows {
		by[r.Date] = r
	}
	if _, ok := by["2026-07-31"]; !ok {
		t.Fatalf("legacy date not normalized: %+v", rows)
	}
	if by["2026-10-28"].Impressions == nil || *by["2026-10-28"].Impressions != 100 {
		t.Fatalf("incoming did not replace same day: %+v", by["2026-10-28"])
	}
	b, err := os.ReadFile(filepath.Join(dir, "latest.json"))
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
	if totals["impressions"] != 101.0 || totals["likes"] != 10.0 {
		t.Fatalf("totals %v", totals)
	}
}

func f64(v float64) *float64 { return &v }

func TestLatestJSONHasNoSecrets(t *testing.T) {
	dir := t.TempDir()
	s := Store{DataDir: dir}
	if err := s.WriteYouTube(youtube.Snapshot{
		FetchedAt: "2026-10-28T00:00:00Z",
		Channel:   youtube.Channel{ID: "UC123", Title: "t"},
		Totals:    youtube.Report{Rows: []map[string]any{{"views": 9.0}}},
	}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "latest.json"))
	if err != nil {
		t.Fatal(err)
	}
	raw := string(b)
	for _, secret := range []string{"refresh_token", "access_token", "client_secret", "token.json"} {
		if contains(raw, secret) {
			t.Fatalf("latest.json leaked %q", secret)
		}
	}
	var l Latest
	if err := json.Unmarshal(b, &l); err != nil {
		t.Fatal(err)
	}
	if l.YouTube == nil || l.YouTube.ChannelID != "UC123" {
		t.Fatalf("latest %+v", l)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || (len(s) > 0 && search(s, sub)))
}
func search(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
