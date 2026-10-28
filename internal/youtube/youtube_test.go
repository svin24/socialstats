package youtube

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPlannedQueriesCoverRequiredMetrics(t *testing.T) {
	qs := PlannedQueries("UCexample000000000000000", "2026-10-01", "2026-10-28")
	need := []string{"totals", "daily", "videos", "traffic", "cards"}
	got := map[string]Query{}
	for _, q := range qs {
		got[q.Name] = q
	}
	for _, n := range need {
		if _, ok := got[n]; !ok {
			t.Fatalf("missing query %s", n)
		}
	}
	core := []string{"views", "estimatedMinutesWatched", "averageViewDuration", "subscribersGained", "subscribersLost", "likes", "comments", "shares"}
	for _, m := range core {
		if !strings.Contains(got["daily"].Metrics, m) {
			t.Fatalf("daily missing %s", m)
		}
	}
	if !strings.Contains(got["traffic"].Dimensions, "insightTrafficSourceType") {
		t.Fatal("traffic sources missing")
	}
	if !got["impressions-thumbnail"].Optional || !got["impressions-legacy"].Optional {
		t.Fatal("impression probes should be optional")
	}
}

func TestQueryOptional400(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"The query is not supported"}}`, 400)
	}))
	defer srv.Close()
	c := &Client{HTTP: srv.Client(), AnalyticsRoot: srv.URL}
	r, err := c.Query(context.Background(), Query{
		Name: "impressions", Optional: true,
		IDs: "channel==UC", StartDate: "2026-10-01", EndDate: "2026-10-02",
		Metrics: "impressions",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Skipped {
		t.Fatalf("expected skip, got %+v", r)
	}
}

func TestQueryParsesRows(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("metrics") != "views" {
			t.Errorf("metrics %s", r.URL.Query().Get("metrics"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"columnHeaders": []any{
				map[string]any{"name": "day"},
				map[string]any{"name": "views"},
			},
			"rows": []any{[]any{"2026-10-01", 12}},
		})
	}))
	defer srv.Close()
	c := &Client{HTTP: srv.Client(), AnalyticsRoot: srv.URL}
	r, err := c.Query(context.Background(), Query{
		Name: "daily", IDs: "channel==UC", StartDate: "2026-10-01", EndDate: "2026-10-01", Metrics: "views", Dimensions: "day",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Rows) != 1 || r.Rows[0]["day"] != "2026-10-01" {
		t.Fatalf("%+v", r)
	}
}

func TestChannelAndVideosHTTP(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/channels", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("id") != "UCexample000000000000000" || r.URL.Query().Get("mine") != "" {
			t.Errorf("explicit channel query = %s", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{map[string]any{
				"id":             "UCexample000000000000000",
				"snippet":        map[string]any{"title": "Example", "customUrl": "@example"},
				"contentDetails": map[string]any{"relatedPlaylists": map[string]any{"uploads": "UUexample000000000000000"}},
				"statistics":     map[string]any{"viewCount": "100", "subscriberCount": "10", "videoCount": "3"},
			}},
		})
	})
	mux.HandleFunc("/videos", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{map[string]any{
				"id":         "vid1",
				"snippet":    map[string]any{"title": "hello", "publishedAt": "2026-10-01T00:00:00Z"},
				"statistics": map[string]any{"viewCount": "9", "likeCount": "2", "commentCount": "1"},
			}},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), DataRoot: srv.URL}
	ch, err := c.Channel(context.Background(), "UCexample000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	if ch.Title != "Example" || ch.SubscriberCount != 10 {
		t.Fatalf("%+v", ch)
	}
	vids, err := c.VideoMeta(context.Background(), []string{"vid1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vids) != 1 || vids[0].Title != "hello" {
		t.Fatalf("%+v", vids)
	}
}

func TestFetchDetectsChannelViaMine(t *testing.T) {
	const detected = "UCdetected000000000000000"
	var sawMine bool
	var analyticsIDs string
	mux := http.NewServeMux()
	mux.HandleFunc("/channels", func(w http.ResponseWriter, r *http.Request) {
		sawMine = r.URL.Query().Get("mine") == "true" && r.URL.Query().Get("id") == ""
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{map[string]any{
				"id":         detected,
				"snippet":    map[string]any{"title": "Example"},
				"statistics": map[string]any{},
			}},
		})
	})
	mux.HandleFunc("/reports", func(w http.ResponseWriter, r *http.Request) {
		if analyticsIDs == "" {
			analyticsIDs = r.URL.Query().Get("ids")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"columnHeaders": []any{map[string]any{"name": "views"}},
			"rows":          []any{},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := &Client{HTTP: srv.Client(), DataRoot: srv.URL, AnalyticsRoot: srv.URL + "/reports"}
	snap, err := Fetch(context.Background(), c, "", "2026-10-01", "2026-10-02", 0)
	if err != nil {
		t.Fatal(err)
	}
	if !sawMine {
		t.Fatal("expected channels?mine=true and no id")
	}
	if snap.Channel.ID != detected {
		t.Fatalf("channel %s", snap.Channel.ID)
	}
	if analyticsIDs != "channel=="+detected {
		t.Fatalf("analytics ids %q", analyticsIDs)
	}
}
