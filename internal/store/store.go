package store

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"socialstats/internal/xcsv"
	"socialstats/internal/youtube"
)

const SchemaVersion = 1

type Latest struct {
	SchemaVersion int            `json:"schemaVersion"`
	GeneratedAt   string         `json:"generatedAt"`
	YouTube       *YouTubeLatest `json:"youtube,omitempty"`
	X             *XLatest       `json:"x,omitempty"`
}

type YouTubeLatest struct {
	ChannelID    string           `json:"channelId"`
	ChannelTitle string           `json:"channelTitle,omitempty"`
	RangeStart   string           `json:"rangeStart"`
	RangeEnd     string           `json:"rangeEnd"`
	Lifetime     youtube.Channel  `json:"lifetime"`
	Totals       map[string]any   `json:"totals,omitempty"`
	DailyCount   int              `json:"dailyCount"`
	VideoCount   int              `json:"videoCount"`
	TopVideos    []map[string]any `json:"topVideos,omitempty"`
	Notes        []string         `json:"notes,omitempty"`
}

type XLatest struct {
	Handle     string  `json:"handle"`
	PostCount  int     `json:"postCount"`
	DailyCount int     `json:"dailyCount"`
	RangeStart string  `json:"rangeStart,omitempty"`
	RangeEnd   string  `json:"rangeEnd,omitempty"`
	Totals     XTotals `json:"totals"`
}

type XTotals struct {
	Impressions   *float64 `json:"impressions,omitempty"`
	Engagements   *float64 `json:"engagements,omitempty"`
	Likes         *float64 `json:"likes,omitempty"`
	Bookmarks     *float64 `json:"bookmarks,omitempty"`
	Shares        *float64 `json:"shares,omitempty"`
	NewFollows    *float64 `json:"newFollows,omitempty"`
	Unfollows     *float64 `json:"unfollows,omitempty"`
	Replies       *float64 `json:"replies,omitempty"`
	Reposts       *float64 `json:"reposts,omitempty"`
	ProfileVisits *float64 `json:"profileVisits,omitempty"`
}

type Store struct {
	DataDir string
}

func (s Store) YouTubeDir() string { return filepath.Join(s.DataDir, "youtube") }
func (s Store) XDir() string       { return filepath.Join(s.DataDir, "x") }

func (s Store) WriteYouTube(snap youtube.Snapshot) error {
	dir := s.YouTubeDir()
	if err := os.MkdirAll(filepath.Join(dir, "snapshots"), 0o755); err != nil {
		return err
	}
	stamp := timestamp(snap.FetchedAt)

	dailyPath := filepath.Join(dir, "daily.json")
	existingDaily, err := readMaps(dailyPath)
	if err != nil {
		return err
	}
	mergedDaily := mergeByKey(existingDaily, snap.Daily.Rows, "day")
	if err := writeJSON(dailyPath, mergedDaily); err != nil {
		return err
	}
	if err := writeMapsCSV(filepath.Join(dir, "daily.csv"), dailyColumns(), mergedDaily); err != nil {
		return err
	}

	videosPath := filepath.Join(dir, "videos.json")
	existingVideos, err := readMaps(videosPath)
	if err != nil {
		return err
	}
	incomingVideos := withRange(snap.Videos.Rows, snap.RangeStart, snap.RangeEnd)
	mergedVideos := mergeByKey(existingVideos, incomingVideos, "video")
	if err := writeJSON(videosPath, mergedVideos); err != nil {
		return err
	}
	if err := writeMapsCSV(filepath.Join(dir, "videos.csv"), videoColumns(), mergedVideos); err != nil {
		return err
	}

	trafficPath := filepath.Join(dir, "traffic_sources.json")
	existingTraffic, err := readMaps(trafficPath)
	if err != nil {
		return err
	}
	mergedTraffic := mergeByComposite(existingTraffic, snap.Traffic.Rows, "day", "insightTrafficSourceType")
	if err := writeJSON(trafficPath, mergedTraffic); err != nil {
		return err
	}
	if err := writeMapsCSV(filepath.Join(dir, "traffic_sources.csv"), trafficColumns(), mergedTraffic); err != nil {
		return err
	}

	impPath := filepath.Join(dir, "impressions.json")
	existingImp, err := readMaps(impPath)
	if err != nil {
		return err
	}
	mergedImp := mergeByKey(existingImp, snap.Impressions.Rows, "day")
	if err := writeJSON(impPath, mergedImp); err != nil {
		return err
	}
	if err := writeMapsCSV(filepath.Join(dir, "impressions.csv"), impressionColumns(), mergedImp); err != nil {
		return err
	}

	cardPath := filepath.Join(dir, "cards.json")
	existingCards, err := readMaps(cardPath)
	if err != nil {
		return err
	}
	mergedCards := mergeByKey(existingCards, snap.Cards.Rows, "day")
	if err := writeJSON(cardPath, mergedCards); err != nil {
		return err
	}
	if err := writeMapsCSV(filepath.Join(dir, "cards.csv"), cardColumns(), mergedCards); err != nil {
		return err
	}

	var totals map[string]any
	if len(snap.Totals.Rows) > 0 {
		totals = snap.Totals.Rows[0]
	}
	channelDoc := map[string]any{
		"channel":    snap.Channel,
		"rangeStart": snap.RangeStart,
		"rangeEnd":   snap.RangeEnd,
		"fetchedAt":  snap.FetchedAt,
		"totals":     totals,
		"notes":      snap.Notes,
	}
	if err := writeJSON(filepath.Join(dir, "channel.json"), channelDoc); err != nil {
		return err
	}

	if err := writeJSON(filepath.Join(dir, "snapshots", stamp+".json"), snap); err != nil {
		return err
	}
	return s.refreshLatest(func(l *Latest) {
		l.YouTube = youtubeSummary(snap, mergedDaily, mergedVideos)
	})
}

func (s Store) WriteX(handle string, posts []xcsv.Post, daily []xcsv.Daily) error {
	dir := s.XDir()
	if err := os.MkdirAll(filepath.Join(dir, "snapshots"), 0o755); err != nil {
		return err
	}
	stamp := time.Now().UTC().Format("20060102T150405Z")

	postsPath := filepath.Join(dir, "posts.json")
	existingPosts, err := readPosts(postsPath)
	if err != nil {
		return err
	}
	mergedPosts := mergePosts(existingPosts, posts)
	if err := writeJSON(postsPath, mergedPosts); err != nil {
		return err
	}
	if err := writePostsCSV(filepath.Join(dir, "posts.csv"), mergedPosts); err != nil {
		return err
	}

	dailyPath := filepath.Join(dir, "daily.json")
	existingDaily, err := readDaily(dailyPath)
	if err != nil {
		return err
	}
	mergedDaily := mergeDaily(existingDaily, daily)
	if err := writeJSON(dailyPath, mergedDaily); err != nil {
		return err
	}
	if err := writeDailyCSV(filepath.Join(dir, "daily.csv"), mergedDaily); err != nil {
		return err
	}

	snap := map[string]any{
		"handle": handle,
		"posts":  posts,
		"daily":  daily,
	}
	if err := writeJSON(filepath.Join(dir, "snapshots", stamp+".json"), snap); err != nil {
		return err
	}
	return s.refreshLatest(func(l *Latest) {
		l.X = xSummary(handle, mergedPosts, mergedDaily)
	})
}

func (s Store) Latest() (Latest, error) {
	var l Latest
	b, err := os.ReadFile(filepath.Join(s.DataDir, "latest.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return Latest{SchemaVersion: SchemaVersion}, nil
		}
		return Latest{}, err
	}
	if err := json.Unmarshal(b, &l); err != nil {
		return Latest{}, err
	}
	return l, nil
}

func (s Store) refreshLatest(mut func(*Latest)) error {
	l, err := s.Latest()
	if err != nil {
		return err
	}
	l.SchemaVersion = SchemaVersion
	l.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	mut(&l)
	if err := writeJSON(filepath.Join(s.DataDir, "latest.json"), l); err != nil {
		return err
	}
	return writeLatestCSV(filepath.Join(s.DataDir, "latest.csv"), l)
}

func youtubeSummary(snap youtube.Snapshot, daily, videos []map[string]any) *YouTubeLatest {
	var totals map[string]any
	if len(snap.Totals.Rows) > 0 {
		totals = snap.Totals.Rows[0]
	}
	top := append([]map[string]any{}, videos...)
	sort.Slice(top, func(i, j int) bool {
		return num(top[i]["views"]) > num(top[j]["views"])
	})
	if len(top) > 10 {
		top = top[:10]
	}
	return &YouTubeLatest{
		ChannelID:    snap.Channel.ID,
		ChannelTitle: snap.Channel.Title,
		RangeStart:   snap.RangeStart,
		RangeEnd:     snap.RangeEnd,
		Lifetime:     snap.Channel,
		Totals:       totals,
		DailyCount:   len(daily),
		VideoCount:   len(videos),
		TopVideos:    top,
		Notes:        snap.Notes,
	}
}

func xSummary(handle string, posts []xcsv.Post, daily []xcsv.Daily) *XLatest {
	xl := &XLatest{Handle: handle, PostCount: len(posts), DailyCount: len(daily)}
	var start, end string
	consider := func(date string) {
		date = xcsv.NormalizeDate(date)
		if date == "" {
			return
		}
		if start == "" || date < start {
			start = date
		}
		if end == "" || date > end {
			end = date
		}
	}
	add := func(dst **float64, v *float64) {
		if v == nil {
			return
		}
		if *dst == nil {
			z := 0.0
			*dst = &z
		}
		**dst += *v
	}
	// Daily account rows are the account-level totals. Post rows fill days
	// the daily series does not already cover, so the two exports are not added twice.
	covered := map[string]bool{}
	for _, d := range daily {
		date := xcsv.NormalizeDate(d.Date)
		if date != "" {
			covered[date] = true
		}
		consider(date)
		add(&xl.Totals.Impressions, d.Impressions)
		add(&xl.Totals.Engagements, d.Engagements)
		add(&xl.Totals.Likes, d.Likes)
		add(&xl.Totals.Bookmarks, d.Bookmarks)
		add(&xl.Totals.Shares, d.Shares)
		add(&xl.Totals.NewFollows, d.Follows)
		add(&xl.Totals.Unfollows, d.Unfollows)
		add(&xl.Totals.Replies, d.Replies)
		add(&xl.Totals.Reposts, d.Reposts)
		add(&xl.Totals.ProfileVisits, d.ProfileClicks)
	}
	for _, p := range posts {
		date := xcsv.NormalizeDate(p.Date)
		consider(date)
		if date != "" && covered[date] {
			continue
		}
		add(&xl.Totals.Impressions, p.Impressions)
		add(&xl.Totals.Engagements, p.Engagements)
		add(&xl.Totals.Likes, p.Likes)
		add(&xl.Totals.Bookmarks, p.Bookmarks)
		add(&xl.Totals.NewFollows, p.Follows)
		add(&xl.Totals.Replies, p.Replies)
		add(&xl.Totals.Reposts, p.Reposts)
		add(&xl.Totals.ProfileVisits, p.ProfileClicks)
	}
	xl.RangeStart = start
	xl.RangeEnd = end
	return xl
}

func mergeByKey(existing, incoming []map[string]any, key string) []map[string]any {
	idx := map[string]int{}
	out := make([]map[string]any, 0, len(existing)+len(incoming))
	for _, row := range existing {
		k := stringify(row[key])
		if k == "" {
			out = append(out, row)
			continue
		}
		idx[k] = len(out)
		out = append(out, row)
	}
	for _, row := range incoming {
		k := stringify(row[key])
		if k == "" {
			out = append(out, row)
			continue
		}
		if i, ok := idx[k]; ok {
			out[i] = row
		} else {
			idx[k] = len(out)
			out = append(out, row)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return stringify(out[i][key]) < stringify(out[j][key])
	})
	return out
}

func mergeByComposite(existing, incoming []map[string]any, keys ...string) []map[string]any {
	comp := func(row map[string]any) string {
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = stringify(row[k])
		}
		return strings.Join(parts, "\x1f")
	}
	idx := map[string]int{}
	out := make([]map[string]any, 0, len(existing)+len(incoming))
	for _, row := range existing {
		k := comp(row)
		idx[k] = len(out)
		out = append(out, row)
	}
	for _, row := range incoming {
		k := comp(row)
		if i, ok := idx[k]; ok {
			out[i] = row
		} else {
			idx[k] = len(out)
			out = append(out, row)
		}
	}
	sort.Slice(out, func(i, j int) bool { return comp(out[i]) < comp(out[j]) })
	return out
}

func mergePosts(existing, incoming []xcsv.Post) []xcsv.Post {
	norm := func(p xcsv.Post) xcsv.Post {
		p.Date = xcsv.NormalizeDate(p.Date)
		return p
	}
	idx := map[string]int{}
	out := make([]xcsv.Post, 0, len(existing)+len(incoming))
	for _, p := range existing {
		p = norm(p)
		if p.ID == "" {
			out = append(out, p)
			continue
		}
		idx[p.ID] = len(out)
		out = append(out, p)
	}
	for _, p := range incoming {
		p = norm(p)
		if p.ID == "" {
			out = append(out, p)
			continue
		}
		if i, ok := idx[p.ID]; ok {
			out[i] = preferPost(out[i], p)
		} else {
			idx[p.ID] = len(out)
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date == out[j].Date {
			return out[i].ID < out[j].ID
		}
		return out[i].Date < out[j].Date
	})
	return out
}

func preferPost(old, neu xcsv.Post) xcsv.Post {
	if numPtr(neu.Impressions) >= numPtr(old.Impressions) {
		return neu
	}
	return old
}

func mergeDaily(existing, incoming []xcsv.Daily) []xcsv.Daily {
	norm := func(d xcsv.Daily) xcsv.Daily {
		d.Date = xcsv.NormalizeDate(d.Date)
		return d
	}
	idx := map[string]int{}
	out := make([]xcsv.Daily, 0, len(existing)+len(incoming))
	add := func(d xcsv.Daily) {
		d = norm(d)
		if d.Date == "" {
			out = append(out, d)
			return
		}
		if i, ok := idx[d.Date]; ok {
			out[i] = d
			return
		}
		idx[d.Date] = len(out)
		out = append(out, d)
	}
	for _, d := range existing {
		add(d)
	}
	for _, d := range incoming {
		add(d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out
}

func withRange(rows []map[string]any, start, end string) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		cp := clone(row)
		cp["rangeStart"] = start
		cp["rangeEnd"] = end
		out = append(out, cp)
	}
	return out
}

func clone(m map[string]any) map[string]any {
	cp := make(map[string]any, len(m)+2)
	for k, v := range m {
		cp[k] = v
	}
	return cp
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func writeMapsCSV(path string, preferred []string, rows []map[string]any) error {
	seen := map[string]bool{}
	cols := append([]string{}, preferred...)
	for _, c := range cols {
		seen[c] = true
	}
	for _, row := range rows {
		for k := range row {
			if !seen[k] {
				seen[k] = true
				cols = append(cols, k)
			}
		}
	}
	records := make([][]string, 0, len(rows)+1)
	records = append(records, cols)
	for _, row := range rows {
		rec := make([]string, len(cols))
		for i, c := range cols {
			rec[i] = stringify(row[c])
		}
		records = append(records, rec)
	}
	return writeCSV(path, records)
}

func writePostsCSV(path string, posts []xcsv.Post) error {
	cols := []string{
		"id", "date", "createdAt", "permalink", "text",
		"impressions", "engagements", "engagementRate",
		"reposts", "replies", "likes", "quotes", "bookmarks",
		"profileClicks", "urlClicks", "hashtagClicks", "detailExpands",
		"permalinkClicks", "follows", "mediaViews", "mediaEngagements", "sourceFile",
	}
	records := [][]string{cols}
	for _, p := range posts {
		records = append(records, []string{
			p.ID, p.Date, p.CreatedAt, p.Permalink, p.Text,
			f(p.Impressions), f(p.Engagements), f(p.EngagementRate),
			f(p.Reposts), f(p.Replies), f(p.Likes), f(p.Quotes), f(p.Bookmarks),
			f(p.ProfileClicks), f(p.URLClicks), f(p.HashtagClicks), f(p.DetailExpands),
			f(p.PermalinkClicks), f(p.Follows), f(p.MediaViews), f(p.MediaEngagements), p.SourceFile,
		})
	}
	return writeCSV(path, records)
}

func writeDailyCSV(path string, daily []xcsv.Daily) error {
	cols := []string{
		"date", "postsPublished", "impressions", "engagements", "engagementRate",
		"reposts", "replies", "likes", "bookmarks", "shares",
		"profileClicks", "urlClicks", "follows", "unfollows",
		"mediaViews", "mediaEngagements", "videoViews", "sourceFile",
	}
	records := [][]string{cols}
	for _, d := range daily {
		records = append(records, []string{
			d.Date, f(d.PostsPublished), f(d.Impressions), f(d.Engagements), f(d.EngagementRate),
			f(d.Reposts), f(d.Replies), f(d.Likes), f(d.Bookmarks), f(d.Shares),
			f(d.ProfileClicks), f(d.URLClicks), f(d.Follows), f(d.Unfollows),
			f(d.MediaViews), f(d.MediaEngagements), f(d.VideoViews), d.SourceFile,
		})
	}
	return writeCSV(path, records)
}

func writeLatestCSV(path string, l Latest) error {
	records := [][]string{{"platform", "key", "value"}}
	records = append(records, []string{"meta", "generatedAt", l.GeneratedAt})
	if y := l.YouTube; y != nil {
		records = append(records,
			[]string{"youtube", "channelId", y.ChannelID},
			[]string{"youtube", "channelTitle", y.ChannelTitle},
			[]string{"youtube", "rangeStart", y.RangeStart},
			[]string{"youtube", "rangeEnd", y.RangeEnd},
			[]string{"youtube", "dailyCount", strconv.Itoa(y.DailyCount)},
			[]string{"youtube", "videoCount", strconv.Itoa(y.VideoCount)},
		)
		for k, v := range y.Totals {
			records = append(records, []string{"youtube", k, stringify(v)})
		}
	}
	if x := l.X; x != nil {
		records = append(records,
			[]string{"x", "handle", x.Handle},
			[]string{"x", "postCount", strconv.Itoa(x.PostCount)},
			[]string{"x", "dailyCount", strconv.Itoa(x.DailyCount)},
			[]string{"x", "rangeStart", x.RangeStart},
			[]string{"x", "rangeEnd", x.RangeEnd},
			[]string{"x", "impressions", f(x.Totals.Impressions)},
			[]string{"x", "engagements", f(x.Totals.Engagements)},
			[]string{"x", "likes", f(x.Totals.Likes)},
			[]string{"x", "bookmarks", f(x.Totals.Bookmarks)},
			[]string{"x", "shares", f(x.Totals.Shares)},
			[]string{"x", "newFollows", f(x.Totals.NewFollows)},
			[]string{"x", "unfollows", f(x.Totals.Unfollows)},
			[]string{"x", "replies", f(x.Totals.Replies)},
			[]string{"x", "reposts", f(x.Totals.Reposts)},
			[]string{"x", "profileVisits", f(x.Totals.ProfileVisits)},
		)
	}
	return writeCSV(path, records)
}

func writeCSV(path string, records [][]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	if err := w.WriteAll(records); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readMaps(path string) ([]map[string]any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var rows []map[string]any
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return rows, nil
}

func readPosts(path string) ([]xcsv.Post, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var rows []xcsv.Post
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func readDaily(path string) ([]xcsv.Daily, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var rows []xcsv.Daily
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func dailyColumns() []string {
	return []string{"day", "views", "estimatedMinutesWatched", "averageViewDuration", "subscribersGained", "subscribersLost", "likes", "comments", "shares"}
}
func videoColumns() []string {
	return []string{"video", "title", "views", "estimatedMinutesWatched", "averageViewDuration", "subscribersGained", "subscribersLost", "likes", "comments", "shares", "rangeStart", "rangeEnd"}
}
func trafficColumns() []string {
	return []string{"day", "insightTrafficSourceType", "views", "estimatedMinutesWatched"}
}
func impressionColumns() []string {
	return []string{"day", "videoThumbnailImpressions", "videoThumbnailImpressionsClickRate", "impressions", "impressionClickThroughRate"}
}
func cardColumns() []string {
	return []string{"day", "cardImpressions", "cardClicks", "cardClickRate", "cardTeaserImpressions", "cardTeaserClicks"}
}

func stringify(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	default:
		return fmt.Sprint(v)
	}
}

func f(v *float64) string {
	if v == nil {
		return ""
	}
	if *v == float64(int64(*v)) {
		return strconv.FormatInt(int64(*v), 10)
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

func num(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case json.Number:
		f, _ := t.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(t, 64)
		return f
	default:
		return 0
	}
}

func numPtr(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func timestamp(rfc3339 string) string {
	if t, err := time.Parse(time.RFC3339, rfc3339); err == nil {
		return t.UTC().Format("20060102T150405Z")
	}
	return time.Now().UTC().Format("20060102T150405Z")
}
