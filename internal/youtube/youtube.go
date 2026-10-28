package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	analyticsBase = "https://youtubeanalytics.googleapis.com/v2/reports"
	dataBase      = "https://www.googleapis.com/youtube/v3"
	userAgent     = "socialstats/0.1.0"
)

type Query struct {
	Name       string
	IDs        string
	StartDate  string
	EndDate    string
	Metrics    string
	Dimensions string
	Filters    string
	Sort       string
	MaxResults int
	Optional   bool
}

type Report struct {
	Query   Query            `json:"query"`
	Columns []string         `json:"columns"`
	Rows    []map[string]any `json:"rows"`
	Warning string           `json:"warning,omitempty"`
	Skipped bool             `json:"skipped,omitempty"`
}

type Channel struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	CustomURL       string `json:"customUrl,omitempty"`
	UploadsID       string `json:"uploadsPlaylistId,omitempty"`
	SubscriberCount int64  `json:"subscriberCount,omitempty"`
	ViewCount       int64  `json:"viewCount,omitempty"`
	VideoCount      int64  `json:"videoCount,omitempty"`
	HiddenSubs      bool   `json:"hiddenSubscriberCount,omitempty"`
}

type Video struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	PublishedAt  string `json:"publishedAt,omitempty"`
	ViewCount    int64  `json:"viewCount,omitempty"`
	LikeCount    int64  `json:"likeCount,omitempty"`
	CommentCount int64  `json:"commentCount,omitempty"`
}

type Snapshot struct {
	FetchedAt   string   `json:"fetchedAt"`
	Channel     Channel  `json:"channel"`
	RangeStart  string   `json:"rangeStart"`
	RangeEnd    string   `json:"rangeEnd"`
	Totals      Report   `json:"totals"`
	Daily       Report   `json:"daily"`
	Videos      Report   `json:"videos"`
	Traffic     Report   `json:"trafficSources"`
	Cards       Report   `json:"cards"`
	Impressions Report   `json:"impressions"`
	VideoMeta   []Video  `json:"videoMeta"`
	Notes       []string `json:"notes,omitempty"`
}

type Client struct {
	HTTP          *http.Client
	AnalyticsRoot string // default https://youtubeanalytics.googleapis.com/v2/reports
	DataRoot      string // default https://www.googleapis.com/youtube/v3
}

func (c *Client) http() *http.Client {
	if c != nil && c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func (c *Client) analyticsRoot() string {
	if c != nil && c.AnalyticsRoot != "" {
		return c.AnalyticsRoot
	}
	return analyticsBase
}

func (c *Client) dataRoot() string {
	if c != nil && c.DataRoot != "" {
		return strings.TrimRight(c.DataRoot, "/")
	}
	return dataBase
}

func PlannedQueries(channelID, start, end string) []Query {
	ids := "channel==" + channelID
	core := "views,estimatedMinutesWatched,averageViewDuration,subscribersGained,subscribersLost,likes,comments,shares"
	return []Query{
		{Name: "totals", IDs: ids, StartDate: start, EndDate: end, Metrics: core},
		{Name: "daily", IDs: ids, StartDate: start, EndDate: end, Metrics: core, Dimensions: "day", Sort: "day"},
		{Name: "cards", IDs: ids, StartDate: start, EndDate: end, Metrics: "cardImpressions,cardClicks,cardClickRate,cardTeaserImpressions,cardTeaserClicks", Dimensions: "day", Sort: "day"},
		{Name: "traffic", IDs: ids, StartDate: start, EndDate: end, Metrics: "views,estimatedMinutesWatched", Dimensions: "day,insightTrafficSourceType", Sort: "day"},
		{Name: "videos", IDs: ids, StartDate: start, EndDate: end, Metrics: core, Dimensions: "video", Sort: "-views", MaxResults: 200},
		{Name: "impressions-thumbnail", IDs: ids, StartDate: start, EndDate: end, Metrics: "videoThumbnailImpressions,videoThumbnailImpressionsClickRate", Dimensions: "day", Sort: "day", Optional: true},
		{Name: "impressions-legacy", IDs: ids, StartDate: start, EndDate: end, Metrics: "impressions,impressionClickThroughRate", Dimensions: "day", Sort: "day", Optional: true},
	}
}

func (c *Client) Query(ctx context.Context, q Query) (Report, error) {
	u, err := url.Parse(c.analyticsRoot())
	if err != nil {
		return Report{}, err
	}
	v := url.Values{}
	v.Set("ids", q.IDs)
	v.Set("startDate", q.StartDate)
	v.Set("endDate", q.EndDate)
	v.Set("metrics", q.Metrics)
	if q.Dimensions != "" {
		v.Set("dimensions", q.Dimensions)
	}
	if q.Filters != "" {
		v.Set("filters", q.Filters)
	}
	if q.Sort != "" {
		v.Set("sort", q.Sort)
	}
	if q.MaxResults > 0 {
		v.Set("maxResults", strconv.Itoa(q.MaxResults))
	}
	u.RawQuery = v.Encode()

	body, status, err := c.get(ctx, u.String())
	if err != nil {
		return Report{}, err
	}
	if status == 400 && q.Optional {
		return Report{Query: q, Skipped: true, Warning: string(body)}, nil
	}
	if status >= 400 {
		return Report{}, fmt.Errorf("youtube analytics %s: HTTP %d: %s", q.Name, status, truncate(string(body), 400))
	}

	var raw struct {
		ColumnHeaders []struct {
			Name string `json:"name"`
		} `json:"columnHeaders"`
		Rows [][]any `json:"rows"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return Report{}, fmt.Errorf("decode analytics %s: %w", q.Name, err)
	}
	cols := make([]string, 0, len(raw.ColumnHeaders))
	for _, h := range raw.ColumnHeaders {
		cols = append(cols, h.Name)
	}
	rows := make([]map[string]any, 0, len(raw.Rows))
	for _, r := range raw.Rows {
		m := map[string]any{}
		for i, col := range cols {
			if i < len(r) {
				m[col] = r[i]
			}
		}
		rows = append(rows, m)
	}
	return Report{Query: q, Columns: cols, Rows: rows}, nil
}

func (c *Client) Channel(ctx context.Context, id string) (Channel, error) {
	q := url.Values{}
	q.Set("part", "snippet,contentDetails,statistics")
	// Empty or "mine" selects the channel owned by the authorized account.
	if id == "" || strings.EqualFold(id, "mine") {
		q.Set("mine", "true")
	} else {
		q.Set("id", id)
	}
	body, err := c.data(ctx, "/channels", q)
	if err != nil {
		return Channel{}, err
	}
	var raw struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				Title     string `json:"title"`
				CustomURL string `json:"customUrl"`
			} `json:"snippet"`
			ContentDetails struct {
				RelatedPlaylists struct {
					Uploads string `json:"uploads"`
				} `json:"relatedPlaylists"`
			} `json:"contentDetails"`
			Statistics struct {
				ViewCount             string `json:"viewCount"`
				SubscriberCount       string `json:"subscriberCount"`
				HiddenSubscriberCount bool   `json:"hiddenSubscriberCount"`
				VideoCount            string `json:"videoCount"`
			} `json:"statistics"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return Channel{}, err
	}
	if len(raw.Items) == 0 {
		return Channel{}, fmt.Errorf("youtube data: no channel returned for %s", id)
	}
	it := raw.Items[0]
	return Channel{
		ID:              it.ID,
		Title:           it.Snippet.Title,
		CustomURL:       it.Snippet.CustomURL,
		UploadsID:       it.ContentDetails.RelatedPlaylists.Uploads,
		SubscriberCount: atoi(it.Statistics.SubscriberCount),
		ViewCount:       atoi(it.Statistics.ViewCount),
		VideoCount:      atoi(it.Statistics.VideoCount),
		HiddenSubs:      it.Statistics.HiddenSubscriberCount,
	}, nil
}

func (c *Client) VideoMeta(ctx context.Context, ids []string) ([]Video, error) {
	var out []Video
	for i := 0; i < len(ids); i += 50 {
		end := i + 50
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[i:end]
		q := url.Values{}
		q.Set("part", "snippet,statistics")
		q.Set("id", strings.Join(chunk, ","))
		body, err := c.data(ctx, "/videos", q)
		if err != nil {
			return out, err
		}
		var raw struct {
			Items []struct {
				ID      string `json:"id"`
				Snippet struct {
					Title       string `json:"title"`
					PublishedAt string `json:"publishedAt"`
				} `json:"snippet"`
				Statistics struct {
					ViewCount    string `json:"viewCount"`
					LikeCount    string `json:"likeCount"`
					CommentCount string `json:"commentCount"`
				} `json:"statistics"`
			} `json:"items"`
		}
		if err := json.Unmarshal(body, &raw); err != nil {
			return out, err
		}
		for _, it := range raw.Items {
			out = append(out, Video{
				ID:           it.ID,
				Title:        it.Snippet.Title,
				PublishedAt:  it.Snippet.PublishedAt,
				ViewCount:    atoi(it.Statistics.ViewCount),
				LikeCount:    atoi(it.Statistics.LikeCount),
				CommentCount: atoi(it.Statistics.CommentCount),
			})
		}
	}
	return out, nil
}

func (c *Client) UploadIDs(ctx context.Context, uploadsPlaylist string, max int) ([]string, error) {
	if uploadsPlaylist == "" || max <= 0 {
		return nil, nil
	}
	var ids []string
	page := ""
	for len(ids) < max {
		q := url.Values{}
		q.Set("part", "contentDetails")
		q.Set("playlistId", uploadsPlaylist)
		q.Set("maxResults", "50")
		if page != "" {
			q.Set("pageToken", page)
		}
		body, err := c.data(ctx, "/playlistItems", q)
		if err != nil {
			return ids, err
		}
		var raw struct {
			NextPageToken string `json:"nextPageToken"`
			Items         []struct {
				ContentDetails struct {
					VideoID string `json:"videoId"`
				} `json:"contentDetails"`
			} `json:"items"`
		}
		if err := json.Unmarshal(body, &raw); err != nil {
			return ids, err
		}
		if len(raw.Items) == 0 {
			break
		}
		for _, it := range raw.Items {
			if it.ContentDetails.VideoID != "" {
				ids = append(ids, it.ContentDetails.VideoID)
				if len(ids) >= max {
					break
				}
			}
		}
		if raw.NextPageToken == "" {
			break
		}
		page = raw.NextPageToken
	}
	return ids, nil
}

func Fetch(ctx context.Context, c *Client, channelID, start, end string, maxVideos int) (Snapshot, error) {
	var notes []string
	// An empty channelID is resolved with channels?mine=true. Later analytics
	// queries use the id from that response, not a built-in default.
	ch, err := c.Channel(ctx, channelID)
	if err != nil {
		return Snapshot{}, err
	}
	if channelID != "" && channelID != "mine" && ch.ID != "" && ch.ID != channelID {
		notes = append(notes, fmt.Sprintf("authenticated channel is %s, config asked for %s", ch.ID, channelID))
	}

	snap := Snapshot{
		FetchedAt:  time.Now().UTC().Format(time.RFC3339),
		Channel:    ch,
		RangeStart: start,
		RangeEnd:   end,
	}

	run := func(q Query) Report {
		r, err := c.Query(ctx, q)
		if err != nil {
			if q.Optional {
				return Report{Query: q, Skipped: true, Warning: err.Error()}
			}
			notes = append(notes, err.Error())
			return Report{Query: q, Skipped: true, Warning: err.Error()}
		}
		if r.Skipped {
			notes = append(notes, fmt.Sprintf("%s not available from the Analytics API: %s", q.Name, truncate(r.Warning, 180)))
		}
		return r
	}

	queries := PlannedQueries(ch.ID, start, end)
	byName := map[string]Query{}
	for _, q := range queries {
		byName[q.Name] = q
	}
	snap.Totals = run(byName["totals"])
	snap.Daily = run(byName["daily"])
	snap.Cards = run(byName["cards"])
	snap.Traffic = run(byName["traffic"])
	snap.Videos = run(byName["videos"])

	imp := run(byName["impressions-thumbnail"])
	if imp.Skipped || len(imp.Rows) == 0 {
		legacy := run(byName["impressions-legacy"])
		if !legacy.Skipped && len(legacy.Rows) > 0 {
			imp = legacy
		} else if imp.Skipped && legacy.Skipped {
			notes = append(notes, "thumbnail impressions/CTR are not exposed on the YouTube Analytics reports.query API; card impressions were still requested")
		}
	}
	snap.Impressions = imp

	videoIDs := uniqueVideoIDs(snap.Videos)
	if ch.UploadsID != "" && maxVideos > 0 {
		uploads, err := c.UploadIDs(ctx, ch.UploadsID, maxVideos)
		if err != nil {
			notes = append(notes, "list uploads: "+err.Error())
		} else {
			need := missing(videoIDs, uploads)
			for i := 0; i < len(need); i += 200 {
				endIDs := i + 200
				if endIDs > len(need) {
					endIDs = len(need)
				}
				q := byName["videos"]
				q.Name = "videos-batch"
				q.Filters = "video==" + strings.Join(need[i:endIDs], ",")
				q.MaxResults = len(need[i:endIDs])
				extra := run(q)
				if !extra.Skipped {
					snap.Videos.Rows = append(snap.Videos.Rows, extra.Rows...)
					for _, col := range extra.Columns {
						if !contains(snap.Videos.Columns, col) {
							snap.Videos.Columns = append(snap.Videos.Columns, col)
						}
					}
				}
			}
			videoIDs = uniqueVideoIDs(snap.Videos)
		}
	}

	if len(videoIDs) > 0 {
		meta, err := c.VideoMeta(ctx, videoIDs)
		if err != nil {
			notes = append(notes, "video meta: "+err.Error())
		} else {
			snap.VideoMeta = meta
			titleByID := map[string]string{}
			for _, v := range meta {
				titleByID[v.ID] = v.Title
			}
			for _, row := range snap.Videos.Rows {
				if id, ok := row["video"].(string); ok {
					if t := titleByID[id]; t != "" {
						row["title"] = t
					}
				}
			}
			if !contains(snap.Videos.Columns, "title") {
				snap.Videos.Columns = append(snap.Videos.Columns, "title")
			}
		}
	}

	snap.Notes = notes
	return snap, nil
}

func uniqueVideoIDs(r Report) []string {
	seen := map[string]bool{}
	var ids []string
	for _, row := range r.Rows {
		id, _ := row["video"].(string)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids
}

func missing(have, all []string) []string {
	set := map[string]bool{}
	for _, h := range have {
		set[h] = true
	}
	var out []string
	for _, a := range all {
		if !set[a] {
			out = append(out, a)
		}
	}
	return out
}

func (c *Client) data(ctx context.Context, path string, q url.Values) ([]byte, error) {
	u := c.dataRoot() + path + "?" + q.Encode()
	body, status, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("youtube data %s: HTTP %d: %s", path, status, truncate(string(body), 400))
	}
	return body, nil
}

func (c *Client) get(ctx context.Context, rawURL string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

func atoi(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func contains(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}
