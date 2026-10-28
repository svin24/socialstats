package xcsv

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type Kind string

const (
	KindPosts   Kind = "posts"
	KindDaily   Kind = "daily"
	KindUnknown Kind = "unknown"
)

type Post struct {
	ID               string   `json:"id"`
	Permalink        string   `json:"permalink,omitempty"`
	Text             string   `json:"text,omitempty"`
	CreatedAt        string   `json:"createdAt,omitempty"`
	Date             string   `json:"date,omitempty"`
	Impressions      *float64 `json:"impressions,omitempty"`
	Engagements      *float64 `json:"engagements,omitempty"`
	EngagementRate   *float64 `json:"engagementRate,omitempty"`
	Reposts          *float64 `json:"reposts,omitempty"`
	Replies          *float64 `json:"replies,omitempty"`
	Likes            *float64 `json:"likes,omitempty"`
	ProfileClicks    *float64 `json:"profileClicks,omitempty"`
	URLClicks        *float64 `json:"urlClicks,omitempty"`
	HashtagClicks    *float64 `json:"hashtagClicks,omitempty"`
	DetailExpands    *float64 `json:"detailExpands,omitempty"`
	PermalinkClicks  *float64 `json:"permalinkClicks,omitempty"`
	Follows          *float64 `json:"follows,omitempty"`
	MediaViews       *float64 `json:"mediaViews,omitempty"`
	MediaEngagements *float64 `json:"mediaEngagements,omitempty"`
	Bookmarks        *float64 `json:"bookmarks,omitempty"`
	Quotes           *float64 `json:"quotes,omitempty"`
	SourceFile       string   `json:"sourceFile,omitempty"`
}

type Daily struct {
	Date             string   `json:"date"`
	PostsPublished   *float64 `json:"postsPublished,omitempty"`
	Impressions      *float64 `json:"impressions,omitempty"`
	Engagements      *float64 `json:"engagements,omitempty"`
	EngagementRate   *float64 `json:"engagementRate,omitempty"`
	Reposts          *float64 `json:"reposts,omitempty"`
	Replies          *float64 `json:"replies,omitempty"`
	Likes            *float64 `json:"likes,omitempty"`
	Bookmarks        *float64 `json:"bookmarks,omitempty"`
	Shares           *float64 `json:"shares,omitempty"`
	ProfileClicks    *float64 `json:"profileClicks,omitempty"`
	URLClicks        *float64 `json:"urlClicks,omitempty"`
	Follows          *float64 `json:"follows,omitempty"`
	Unfollows        *float64 `json:"unfollows,omitempty"`
	MediaViews       *float64 `json:"mediaViews,omitempty"`
	MediaEngagements *float64 `json:"mediaEngagements,omitempty"`
	VideoViews       *float64 `json:"videoViews,omitempty"`
	SourceFile       string   `json:"sourceFile,omitempty"`
}

type Result struct {
	Kind        Kind     `json:"kind"`
	Path        string   `json:"path"`
	SHA256      string   `json:"sha256"`
	Headers     []string `json:"headers"`
	Posts       []Post   `json:"posts,omitempty"`
	Daily       []Daily  `json:"daily,omitempty"`
	SkippedRows int      `json:"skippedRows,omitempty"`
}

var watchPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^account_analytics.*\.csv$`),
	regexp.MustCompile(`(?i)^account_overview.*\.csv$`),
	regexp.MustCompile(`(?i)^tweet_activity.*\.csv$`),
	regexp.MustCompile(`(?i)^post_activity.*\.csv$`),
	regexp.MustCompile(`(?i)^.*twitter.*analytics.*\.csv$`),
	regexp.MustCompile(`(?i)^.*x[-_ ]?analytics.*\.csv$`),
	regexp.MustCompile(`(?i)^analytics.*\.csv$`),
}

func MatchesWatchName(name string) bool {
	base := filepath.Base(name)
	for _, re := range watchPatterns {
		if re.MatchString(base) {
			return true
		}
	}
	return false
}

func WatchCandidates(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if MatchesWatchName(e.Name()) {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	return out, nil
}

func ParseFile(path string) (Result, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Result{}, err
	}
	sum := sha256.Sum256(b)
	res, err := Parse(bytes.NewReader(b), filepath.Base(path))
	if err != nil {
		return Result{}, err
	}
	res.Path = path
	res.SHA256 = hex.EncodeToString(sum[:])
	return res, nil
}

func Parse(r io.Reader, source string) (Result, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return Result{}, err
	}
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	if len(bytes.TrimSpace(raw)) == 0 {
		return Result{}, fmt.Errorf("%s: empty CSV", source)
	}
	delim := detectDelim(raw)
	cr := csv.NewReader(bytes.NewReader(raw))
	cr.Comma = delim
	cr.LazyQuotes = true
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true

	records, err := cr.ReadAll()
	if err != nil {
		return Result{}, fmt.Errorf("%s: %w", source, err)
	}
	if len(records) < 2 {
		return Result{}, fmt.Errorf("%s: need a header row and at least one data row", source)
	}

	headers := records[0]
	norm := make([]string, len(headers))
	for i, h := range headers {
		norm[i] = normalizeHeader(h)
	}
	idx := indexHeaders(norm)

	res := Result{Headers: headers, Path: source}
	hasPost := hasAny(idx, "id", "permalink", "text")
	hasDate := hasAny(idx, "date")

	switch {
	case hasPost:
		res.Kind = KindPosts
		for _, rec := range records[1:] {
			p, ok := rowToPost(rec, idx, source)
			if !ok {
				res.SkippedRows++
				continue
			}
			res.Posts = append(res.Posts, p)
		}
	case hasDate:
		res.Kind = KindDaily
		for _, rec := range records[1:] {
			d, ok := rowToDaily(rec, idx, source)
			if !ok {
				res.SkippedRows++
				continue
			}
			res.Daily = append(res.Daily, d)
		}
	default:
		res.Kind = KindUnknown
		return res, fmt.Errorf("%s: unrecognized X analytics CSV; headers: %s", source, strings.Join(headers, ", "))
	}
	if res.Kind == KindPosts && len(res.Posts) == 0 {
		return res, fmt.Errorf("%s: no post rows parsed; headers: %s", source, strings.Join(headers, ", "))
	}
	if res.Kind == KindDaily && len(res.Daily) == 0 {
		return res, fmt.Errorf("%s: no daily rows parsed; headers: %s", source, strings.Join(headers, ", "))
	}
	return res, nil
}

func detectDelim(raw []byte) rune {
	line := raw
	if i := bytes.IndexByte(raw, '\n'); i >= 0 {
		line = raw[:i]
	}
	line = bytes.TrimRight(line, "\r")
	counts := map[rune]int{',': 0, ';': 0, '\t': 0}
	inQuotes := false
	for _, r := range string(line) {
		if r == '"' {
			inQuotes = !inQuotes
			continue
		}
		if inQuotes {
			continue
		}
		if _, ok := counts[r]; ok {
			counts[r]++
		}
	}
	best, n := ',', -1
	for r, c := range counts {
		if c > n {
			best, n = r, c
		}
	}
	if n <= 0 {
		return ','
	}
	return best
}

func normalizeHeader(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevSpace = false
			continue
		}
		if !prevSpace {
			b.WriteByte(' ')
			prevSpace = true
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func indexHeaders(norm []string) map[string]int {
	idx := map[string]int{}
	for i, h := range norm {
		for _, key := range matchKey(h) {
			if _, exists := idx[key]; !exists {
				idx[key] = i
			}
		}
	}
	return idx
}

func matchKey(h string) []string {
	h = strings.TrimSpace(h)
	switch h {
	case "tweet id", "post id", "id", "status id", "tweetid", "postid":
		if h == "id" {
			return []string{"id"}
		}
		return []string{"id"}
	case "tweet permalink", "post permalink", "permalink", "url", "link":
		return []string{"permalink"}
	case "tweet text", "post text", "text", "tweet", "post":
		return []string{"text"}
	case "time", "date", "tweet date", "post date", "create date", "created at", "createdat", "datetime":
		return []string{"date"}
	case "impressions", "views", "organic impressions":
		return []string{"impressions"}
	case "engagements", "organic engagements":
		return []string{"engagements"}
	case "engagement rate", "engagementrate":
		return []string{"engagement_rate"}
	case "retweets", "reposts", "retweets organic", "reposts organic":
		return []string{"reposts"}
	case "replies", "replies organic":
		return []string{"replies"}
	case "likes", "favorites", "likes organic":
		return []string{"likes"}
	case "user profile clicks", "profile visits", "profile clicks", "user profile clicks organic":
		return []string{"profile_clicks"}
	case "url clicks", "link clicks", "url clicks organic":
		return []string{"url_clicks"}
	case "hashtag clicks":
		return []string{"hashtag_clicks"}
	case "detail expands":
		return []string{"detail_expands"}
	case "permalink clicks":
		return []string{"permalink_clicks"}
	case "follows", "new follows", "followers":
		return []string{"follows"}
	case "unfollows", "unfollow":
		return []string{"unfollows"}
	case "shares", "share":
		return []string{"shares"}
	case "media views":
		return []string{"media_views"}
	case "video views":
		return []string{"video_views"}
	case "tweets published", "posts published", "tweets", "posts", "create post", "create posts":
		return []string{"posts_published"}
	case "media engagements":
		return []string{"media_engagements"}
	case "bookmarks":
		return []string{"bookmarks"}
	case "quotes", "quote tweets":
		return []string{"quotes"}
	}
	// Fuzzy contains matches for slightly renamed exports.
	switch {
	case strings.Contains(h, "permalink"):
		return []string{"permalink"}
	case strings.Contains(h, "engagement rate"):
		return []string{"engagement_rate"}
	case strings.Contains(h, "profile click") || strings.Contains(h, "profile visit"):
		return []string{"profile_clicks"}
	case strings.Contains(h, "url click") || strings.Contains(h, "link click"):
		return []string{"url_clicks"}
	case strings.Contains(h, "hashtag"):
		return []string{"hashtag_clicks"}
	case strings.Contains(h, "detail expand"):
		return []string{"detail_expands"}
	case h == "tweet id" || strings.HasSuffix(h, " tweet id") || strings.HasSuffix(h, " post id"):
		return []string{"id"}
	case strings.Contains(h, "impression") && !strings.Contains(h, "promoted"):
		return []string{"impressions"}
	}
	return nil
}

func hasAny(idx map[string]int, keys ...string) bool {
	for _, k := range keys {
		if _, ok := idx[k]; ok {
			return true
		}
	}
	return false
}

func cell(rec []string, idx map[string]int, key string) string {
	i, ok := idx[key]
	if !ok || i < 0 || i >= len(rec) {
		return ""
	}
	return strings.TrimSpace(rec[i])
}

func rowToPost(rec []string, idx map[string]int, source string) (Post, bool) {
	p := Post{
		ID:               cell(rec, idx, "id"),
		Permalink:        cell(rec, idx, "permalink"),
		Text:             cell(rec, idx, "text"),
		CreatedAt:        cell(rec, idx, "date"),
		Impressions:      parseNum(cell(rec, idx, "impressions")),
		Engagements:      parseNum(cell(rec, idx, "engagements")),
		EngagementRate:   parseRate(cell(rec, idx, "engagement_rate")),
		Reposts:          parseNum(cell(rec, idx, "reposts")),
		Replies:          parseNum(cell(rec, idx, "replies")),
		Likes:            parseNum(cell(rec, idx, "likes")),
		ProfileClicks:    parseNum(cell(rec, idx, "profile_clicks")),
		URLClicks:        parseNum(cell(rec, idx, "url_clicks")),
		HashtagClicks:    parseNum(cell(rec, idx, "hashtag_clicks")),
		DetailExpands:    parseNum(cell(rec, idx, "detail_expands")),
		PermalinkClicks:  parseNum(cell(rec, idx, "permalink_clicks")),
		Follows:          parseNum(cell(rec, idx, "follows")),
		MediaViews:       parseNum(cell(rec, idx, "media_views")),
		MediaEngagements: parseNum(cell(rec, idx, "media_engagements")),
		Bookmarks:        parseNum(cell(rec, idx, "bookmarks")),
		Quotes:           parseNum(cell(rec, idx, "quotes")),
		SourceFile:       source,
	}
	if p.ID == "" && p.Permalink != "" {
		p.ID = idFromPermalink(p.Permalink)
	}
	if p.CreatedAt != "" {
		p.Date = datePart(p.CreatedAt)
	}
	if p.ID == "" && p.Text == "" && p.Date == "" {
		return Post{}, false
	}
	if p.ID == "" {
		p.ID = fallbackID(p)
	}
	return p, true
}

func rowToDaily(rec []string, idx map[string]int, source string) (Daily, bool) {
	raw := cell(rec, idx, "date")
	d := Daily{
		Date:             datePart(raw),
		PostsPublished:   parseNum(cell(rec, idx, "posts_published")),
		Impressions:      parseNum(cell(rec, idx, "impressions")),
		Engagements:      parseNum(cell(rec, idx, "engagements")),
		EngagementRate:   parseRate(cell(rec, idx, "engagement_rate")),
		Reposts:          parseNum(cell(rec, idx, "reposts")),
		Replies:          parseNum(cell(rec, idx, "replies")),
		Likes:            parseNum(cell(rec, idx, "likes")),
		Bookmarks:        parseNum(cell(rec, idx, "bookmarks")),
		Shares:           parseNum(cell(rec, idx, "shares")),
		ProfileClicks:    parseNum(cell(rec, idx, "profile_clicks")),
		URLClicks:        parseNum(cell(rec, idx, "url_clicks")),
		Follows:          parseNum(cell(rec, idx, "follows")),
		Unfollows:        parseNum(cell(rec, idx, "unfollows")),
		MediaViews:       parseNum(cell(rec, idx, "media_views")),
		MediaEngagements: parseNum(cell(rec, idx, "media_engagements")),
		VideoViews:       parseNum(cell(rec, idx, "video_views")),
		SourceFile:       source,
	}
	if d.Date == "" {
		return Daily{}, false
	}
	return d, true
}

func parseNum(s string) *float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" || strings.EqualFold(s, "n/a") || strings.EqualFold(s, "na") {
		return nil
	}
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSuffix(s, "%")
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &f
}

func parseRate(s string) *float64 {
	orig := strings.TrimSpace(s)
	pct := strings.Contains(orig, "%")
	n := parseNum(orig)
	if n == nil {
		return nil
	}
	v := *n
	if pct || v > 1 {
		v = v / 100
	}
	return &v
}

// NormalizeDate converts known X analytics timestamps to YYYY-MM-DD.
// Unrecognized values are returned trimmed.
func NormalizeDate(s string) string {
	return datePart(s)
}

func datePart(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if len(s) >= 10 {
		if _, err := time.Parse("2006-01-02", s[:10]); err == nil {
			return s[:10]
		}
	}
	// Account overview exports use "Wed, Oct 28, 2026" (and "Wed, Sep 9, 2026").
	// The weekday is part of the value, so a string min/max is not chronological.
	for _, layout := range []string{
		"Mon, Jan 2, 2006",
		"Monday, January 2, 2006",
		time.RFC3339,
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04 +0000",
		"2006-01-02 15:04:05",
		"1/2/2006",
		"01/02/2006",
		"2 Jan 2006",
		time.RFC1123,
	} {
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return t.Format(time.DateOnly)
		}
	}
	return s
}

func idFromPermalink(p string) string {
	p = strings.TrimRight(p, "/")
	if i := strings.LastIndex(p, "/"); i >= 0 && i+1 < len(p) {
		return p[i+1:]
	}
	return ""
}

func fallbackID(p Post) string {
	h := sha256.Sum256([]byte(p.CreatedAt + "\n" + p.Text))
	return "anon-" + hex.EncodeToString(h[:8])
}
