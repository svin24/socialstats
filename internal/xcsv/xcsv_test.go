package xcsv

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func testdata(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join("..", "..", "testdata", name)
	if _, err := os.Stat(p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParsePostActivity(t *testing.T) {
	res, err := ParseFile(testdata(t, "x_post_activity.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Kind != KindPosts {
		t.Fatalf("kind %s", res.Kind)
	}
	if len(res.Posts) != 2 {
		t.Fatalf("posts %d", len(res.Posts))
	}
	p := res.Posts[0]
	if p.ID != "1234567890123456789" {
		t.Fatalf("id %s", p.ID)
	}
	if p.Date != "2026-10-01" {
		t.Fatalf("date %s", p.Date)
	}
	if p.Impressions == nil || *p.Impressions != 1000 {
		t.Fatalf("impressions %v", p.Impressions)
	}
	if p.EngagementRate == nil || *p.EngagementRate != 0.05 {
		t.Fatalf("rate %v", p.EngagementRate)
	}
	if p.Text != "hello, world" {
		t.Fatalf("text %q", p.Text)
	}
}

func TestParseAccountOverview(t *testing.T) {
	res, err := ParseFile(testdata(t, "x_account_overview.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Kind != KindDaily {
		t.Fatalf("kind %s", res.Kind)
	}
	if len(res.Daily) != 3 {
		t.Fatalf("daily %d", len(res.Daily))
	}
	byDate := map[string]map[string]any{}
	for _, d := range res.Daily {
		b, err := json.Marshal(d)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		byDate[d.Date] = m
	}
	// Weekday strings must not survive. "Wed, Sep 9, 2026" is a single-digit day.
	for _, want := range []string{"2026-10-28", "2026-09-09", "2026-07-31"} {
		if _, ok := byDate[want]; !ok {
			t.Fatalf("missing ISO date %s in %+v", want, byDate)
		}
	}
	oct := byDate["2026-10-28"]
	checks := map[string]float64{
		"impressions":    100,
		"likes":          10,
		"engagements":    12,
		"bookmarks":      1,
		"shares":         2,
		"follows":        3,
		"unfollows":      4,
		"replies":        5,
		"reposts":        6,
		"profileClicks":  7,
		"postsPublished": 1,
		"videoViews":     8,
		"mediaViews":     9,
	}
	for k, want := range checks {
		got, ok := oct[k].(float64)
		if !ok || got != want {
			t.Fatalf("%s = %#v, want %v", k, oct[k], want)
		}
	}
}

func TestParseAccountAnalyticsDaily(t *testing.T) {
	res, err := ParseFile(testdata(t, "x_account_analytics.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Kind != KindDaily {
		t.Fatalf("kind %s", res.Kind)
	}
	if len(res.Daily) != 2 {
		t.Fatalf("daily %d", len(res.Daily))
	}
	if res.Daily[0].Date != "2026-10-01" {
		t.Fatalf("date %s", res.Daily[0].Date)
	}
	if res.Daily[1].EngagementRate == nil || *res.Daily[1].EngagementRate != 0.05 {
		t.Fatalf("percent rate %v", res.Daily[1].EngagementRate)
	}
}

func TestParseSemicolon(t *testing.T) {
	res, err := ParseFile(testdata(t, "x_semicolon.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Kind != KindDaily || len(res.Daily) != 2 {
		t.Fatalf("%s %d", res.Kind, len(res.Daily))
	}
	if res.Daily[0].Impressions == nil || *res.Daily[0].Impressions != 100 {
		t.Fatalf("impressions %v", res.Daily[0].Impressions)
	}
}

func TestParseBOMQuotedNewline(t *testing.T) {
	res, err := ParseFile(testdata(t, "x_bom_quoted.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Kind != KindPosts || len(res.Posts) != 1 {
		t.Fatalf("%s %d errheaders=%v", res.Kind, len(res.Posts), res.Headers)
	}
	if res.Posts[0].ID != "999" {
		t.Fatalf("id %s headers=%v", res.Posts[0].ID, res.Headers)
	}
	if res.Posts[0].Impressions == nil || *res.Posts[0].Impressions != 42 {
		t.Fatalf("impressions %v", res.Posts[0].Impressions)
	}
}

func TestUnknownHeaders(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "weird.csv")
	if err := os.WriteFile(tmp, []byte("foo,bar\n1,2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ParseFile(tmp)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestWatchPatterns(t *testing.T) {
	yes := []string{
		"account_analytics.csv",
		"account_analytics_2026-10-28.csv",
		"account_overview_analytics.csv",
		"tweet_activity_metrics.csv",
		"post_activity.csv",
		"my-x-analytics.csv",
		"twitter_analytics_export.csv",
	}
	no := []string{"notes.csv", "photos.png", "account_analytics.txt"}
	for _, n := range yes {
		if !MatchesWatchName(n) {
			t.Errorf("expected match %s", n)
		}
	}
	for _, n := range no {
		if MatchesWatchName(n) {
			t.Errorf("unexpected match %s", n)
		}
	}
}

func TestWatchCandidates(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(name string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("Date,impressions\n2026-10-01,1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("account_analytics.csv")
	mustWrite("notes.csv")
	found, err := WatchCandidates(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || filepath.Base(found[0]) != "account_analytics.csv" {
		t.Fatalf("found %v", found)
	}
}

func TestSHA256Stable(t *testing.T) {
	a, err := ParseFile(testdata(t, "x_post_activity.csv"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := ParseFile(testdata(t, "x_post_activity.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if a.SHA256 == "" || a.SHA256 != b.SHA256 {
		t.Fatalf("sha %s vs %s", a.SHA256, b.SHA256)
	}
}
