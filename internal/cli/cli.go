package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"socialstats/internal/config"
	"socialstats/internal/daterange"
	"socialstats/internal/oauth"
	"socialstats/internal/state"
	"socialstats/internal/store"
	"socialstats/internal/webhook"
	"socialstats/internal/xcsv"
	"socialstats/internal/youtube"
)

const usage = `socialstats — a tool for agents that collects your own YouTube and X creator analytics into local JSON/CSV files.

Usage:
  socialstats [global flags] <command> [command flags]

Commands:
  setup      Print first-time setup, create the config directory, write config.json
  auth       Authorize a Google Desktop OAuth client (loopback) and store a refresh token
  fetch      Pull YouTube Analytics and import any new X CSVs from the watch directory
  import-x   Normalize an X analytics CSV export (or watch ~/Downloads)
  status     Show config/data paths, auth state, and last run

  setup is also available as init.

Global flags:
  --config-dir DIR     Config directory (default: $XDG_CONFIG_HOME/socialstats)
  --data-dir DIR       Data directory (default: $XDG_DATA_HOME/socialstats)
  --channel-id ID      YouTube channel id (default: auto-detect with channels?mine=true)
  --x-handle HANDLE    Optional X handle label (no default)
  --dry-run            Print the plan; do not call Google, write files, or POST webhooks
  --version            Print version and exit
  -h, --help           Show this help

Environment:
  SOCIALSTATS_CONFIG_DIR, SOCIALSTATS_DATA_DIR
  SOCIALSTATS_WEBHOOK_URL, SOCIALSTATS_WEBHOOK_URL_FILE
  SOCIALSTATS_CHANNEL_ID, SOCIALSTATS_X_HANDLE, SOCIALSTATS_X_WATCH_DIR
  SOCIALSTATS_CLIENT_SECRET

Secrets and personal config stay in the config directory ($XDG_CONFIG_HOME/socialstats).
Nothing secret is hardcoded. The YouTube channel id is detected from the authorized
account when it is not set. The X handle is an optional label.
`

type globalFlags struct {
	configDir        string
	dataDir          string
	dryRun           bool
	channelID        string
	xHandle          string
	xWatchDir        string
	webhookURL       string
	webhookURLFile   string
	clientSecretPath string
}

func Main(args []string) int {
	return MainIO(args, os.Stdout, os.Stderr)
}

func MainIO(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		args = []string{"socialstats"}
	}
	gf, rest, err := parseGlobal(args[1:])
	if err == flag.ErrHelp {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if len(rest) == 0 {
		fmt.Fprint(stdout, usage)
		return 0
	}
	cmd, rest := rest[0], rest[1:]
	switch cmd {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	case "version", "--version":
		fmt.Fprintln(stdout, config.Version)
		return 0
	case "setup", "init":
		return runSetup(gf, rest, stdout, stderr)
	case "auth":
		return runAuth(gf, rest, stdout, stderr)
	case "fetch":
		return runFetch(gf, rest, stdout, stderr)
	case "import-x":
		return runImportX(gf, rest, stdout, stderr)
	case "status":
		return runStatus(gf, rest, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n", cmd)
		fmt.Fprint(stderr, usage)
		return 2
	}
}

func parseGlobal(args []string) (globalFlags, []string, error) {
	var gf globalFlags
	fs := flag.NewFlagSet("socialstats", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&gf.configDir, "config-dir", "", "")
	fs.StringVar(&gf.dataDir, "data-dir", "", "")
	fs.BoolVar(&gf.dryRun, "dry-run", false, "")
	fs.StringVar(&gf.channelID, "channel-id", "", "")
	fs.StringVar(&gf.xHandle, "x-handle", "", "")
	fs.StringVar(&gf.xWatchDir, "watch-dir", "", "")
	fs.StringVar(&gf.webhookURL, "webhook-url", "", "")
	fs.StringVar(&gf.webhookURLFile, "webhook-url-file", "", "")
	fs.StringVar(&gf.clientSecretPath, "client-secret", "", "")
	help := fs.Bool("help", false, "")
	h := fs.Bool("h", false, "")
	ver := fs.Bool("version", false, "")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return gf, nil, flag.ErrHelp
		}
		return gf, nil, err
	}
	if *help || *h {
		return gf, nil, flag.ErrHelp
	}
	rest := fs.Args()
	if *ver && len(rest) == 0 {
		return gf, []string{"version"}, nil
	}
	return gf, rest, nil
}

func loadCfg(gf globalFlags) (config.Config, error) {
	return config.Load(config.Overrides{
		ConfigDir:        gf.configDir,
		DataDir:          gf.dataDir,
		YouTubeChannelID: gf.channelID,
		XHandle:          gf.xHandle,
		XWatchDir:        gf.xWatchDir,
		WebhookURL:       gf.webhookURL,
		WebhookURLFile:   gf.webhookURLFile,
		ClientSecretPath: gf.clientSecretPath,
		DryRun:           gf.dryRun,
	})
}

func runAuth(gf globalFlags, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("auth", flag.ContinueOnError)
	fs.SetOutput(stderr)
	open := fs.Bool("open-browser", true, "open the consent URL with xdg-open")
	fs.BoolVar(&gf.dryRun, "dry-run", gf.dryRun, "")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, err := loadCfg(gf)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "OAuth client: %s (exists=%v)\n", cfg.ClientSecretPath, cfg.HasClientSecret())
	fmt.Fprintf(stdout, "Token file:   %s (exists=%v)\n", cfg.TokenPath, cfg.HasToken())
	fmt.Fprintf(stdout, "Scopes:       %s\n", strings.Join(oauth.Scopes, " "))
	if !cfg.HasClientSecret() {
		fmt.Fprintln(stderr, firstTimeSetup(cfg.ClientSecretPath))
		if !cfg.DryRun {
			return 1
		}
	}
	if cfg.DryRun {
		fmt.Fprintln(stdout, "dry-run: would start a 127.0.0.1 loopback listener and wait for Google's redirect.")
		return 0
	}
	if err := cfg.EnsureDirs(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	ctx := context.Background()
	if err := oauth.Login(ctx, cfg.ClientSecretPath, cfg.TokenPath, oauth.AuthOptions{
		OpenBrowser: *open,
		Printf: func(format string, a ...any) {
			fmt.Fprintf(stdout, format, a...)
		},
	}); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func runFetch(gf globalFlags, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("fetch", flag.ContinueOnError)
	fs.SetOutput(stderr)
	since := fs.String("since", "", "start date YYYY-MM-DD")
	until := fs.String("until", "", "end date YYYY-MM-DD")
	days := fs.Int("days", 0, "window length in days (default 28, or incremental since last run)")
	maxVideos := fs.Int("max-videos", 400, "max uploads to request per-video metrics for")
	skipX := fs.Bool("skip-x", false, "do not scan the X CSV watch directory")
	fs.BoolVar(&gf.dryRun, "dry-run", gf.dryRun, "")
	fs.StringVar(&gf.channelID, "channel-id", gf.channelID, "")
	fs.StringVar(&gf.webhookURL, "webhook-url", gf.webhookURL, "")
	fs.StringVar(&gf.webhookURLFile, "webhook-url-file", gf.webhookURLFile, "")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, err := loadCfg(gf)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	st, err := state.Load(cfg.StatePath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	rng, err := daterange.Resolve(time.Now(), *since, *until, *days, st.YouTube.LastEnd)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	fmt.Fprintf(stdout, "config dir: %s\n", cfg.ConfigDir)
	fmt.Fprintf(stdout, "data dir:   %s\n", cfg.DataDir)
	fmt.Fprintf(stdout, "channel:    %s\n", displayChannel(cfg.YouTubeChannelID))
	fmt.Fprintf(stdout, "range:      %s → %s (%d days)\n", rng.StartStr(), rng.EndStr(), rng.Days())
	fmt.Fprintf(stdout, "oauth:      client=%v token=%v\n", cfg.HasClientSecret(), cfg.HasToken())
	fmt.Fprintf(stdout, "webhook:    %s\n", webhookStatus(cfg))
	fmt.Fprintln(stdout, "planned YouTube Analytics queries:")
	planChannel := cfg.YouTubeChannelID
	if planChannel == "" {
		planChannel = "MINE"
		fmt.Fprintln(stdout, "  channel id is resolved with channels?mine=true before the queries run")
	}
	for _, q := range youtube.PlannedQueries(planChannel, rng.StartStr(), rng.EndStr()) {
		opt := ""
		if q.Optional {
			opt = " (optional; skipped if the API rejects it)"
		}
		fmt.Fprintf(stdout, "  - %s metrics=%s dimensions=%s%s\n", q.Name, q.Metrics, q.Dimensions, opt)
	}

	if !cfg.HasClientSecret() {
		fmt.Fprintln(stderr, firstTimeSetup(cfg.ClientSecretPath))
		if !cfg.DryRun {
			return 1
		}
	}

	if cfg.DryRun {
		if !*skipX {
			fmt.Fprintf(stdout, "dry-run: would also scan %s for X analytics CSVs.\n", cfg.XWatchDir)
		}
		fmt.Fprintln(stdout, "dry-run: no Google calls, no files written, no webhook POST.")
		return 0
	}

	if !cfg.HasToken() {
		fmt.Fprintln(stderr, "YouTube fetch needs a refresh token. Run: socialstats auth")
		return 1
	}
	if err := cfg.EnsureDirs(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	httpClient, err := oauth.HTTPClient(ctx, cfg.ClientSecretPath, cfg.TokenPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	yt := &youtube.Client{HTTP: httpClient}
	snap, err := youtube.Fetch(ctx, yt, cfg.YouTubeChannelID, rng.StartStr(), rng.EndStr(), *maxVideos)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	stStore := store.Store{DataDir: cfg.DataDir}
	if err := stStore.WriteYouTube(snap); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	st.MarkYouTube(snap.Channel.ID, rng.StartStr(), rng.EndStr(), time.Now())
	if err := state.Save(cfg.StatePath, st); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "youtube: wrote %s (daily rows=%d videos=%d)\n", filepath.Join(cfg.DataDir, "youtube"), len(snap.Daily.Rows), len(snap.Videos.Rows))
	for _, n := range snap.Notes {
		fmt.Fprintf(stdout, "note: %s\n", n)
	}

	if !*skipX {
		if code := importWatch(cfg, &st, stdout, stderr); code != 0 {
			return code
		}
		if err := state.Save(cfg.StatePath, st); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}

	latest, err := stStore.Latest()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	res, err := webhook.Post(ctx, cfg.WebhookURL, latest, false)
	if err != nil {
		fmt.Fprintf(stderr, "webhook: %v\n", err)
		return 1
	}
	if res.Skipped {
		fmt.Fprintf(stdout, "webhook: skipped (%s)\n", res.Reason)
	} else {
		fmt.Fprintf(stdout, "webhook: POST %s → HTTP %d\n", res.URL, res.StatusCode)
	}
	return 0
}

func runImportX(gf globalFlags, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("import-x", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.BoolVar(&gf.dryRun, "dry-run", gf.dryRun, "")
	fs.StringVar(&gf.xWatchDir, "watch-dir", gf.xWatchDir, "")
	noWatch := fs.Bool("no-watch", false, "do not scan the watch directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	files := fs.Args()
	cfg, err := loadCfg(gf)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	st, err := state.Load(cfg.StatePath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	var paths []string
	paths = append(paths, files...)
	if len(files) == 0 && !*noWatch {
		found, err := xcsv.WatchCandidates(cfg.XWatchDir)
		if err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(stderr, "watch dir %s: %v\n", cfg.XWatchDir, err)
			return 1
		}
		paths = append(paths, found...)
		if len(paths) == 0 {
			fmt.Fprintf(stdout, "no X analytics CSVs in %s\nExport from https://x.com/i/account_analytics (Posts → Export) and re-run, or pass a file path.\n", cfg.XWatchDir)
			return 0
		}
	}
	if len(paths) == 0 {
		fmt.Fprintln(stderr, "import-x: pass a CSV path or omit the path to scan the watch directory")
		return 2
	}

	var allPosts []xcsv.Post
	var allDaily []xcsv.Daily
	imported := 0
	for _, p := range paths {
		res, err := xcsv.ParseFile(p)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", p, err)
			return 1
		}
		if st.SeenX(res.SHA256) {
			fmt.Fprintf(stdout, "skip (already imported): %s\n", p)
			continue
		}
		fmt.Fprintf(stdout, "%s: kind=%s posts=%d daily=%d skipped_rows=%d\n", p, res.Kind, len(res.Posts), len(res.Daily), res.SkippedRows)
		if cfg.DryRun {
			continue
		}
		allPosts = append(allPosts, res.Posts...)
		allDaily = append(allDaily, res.Daily...)
		st.MarkX(state.ImportedFile{
			Path:       p,
			SHA256:     res.SHA256,
			ImportedAt: time.Now().UTC().Format(time.RFC3339),
			Rows:       len(res.Posts) + len(res.Daily),
		}, time.Now())
		imported++
	}
	if cfg.DryRun {
		fmt.Fprintln(stdout, "dry-run: parsed CSVs, no files written.")
		return 0
	}
	if imported == 0 && len(allPosts) == 0 && len(allDaily) == 0 {
		fmt.Fprintln(stdout, "nothing new to import")
		return 0
	}
	if err := cfg.EnsureDirs(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	stStore := store.Store{DataDir: cfg.DataDir}
	if err := stStore.WriteX(cfg.XHandle, allPosts, allDaily); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := state.Save(cfg.StatePath, st); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "x: wrote %s\n", filepath.Join(cfg.DataDir, "x"))

	latest, err := stStore.Latest()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	res, err := webhook.Post(context.Background(), cfg.WebhookURL, latest, false)
	if err != nil {
		fmt.Fprintf(stderr, "webhook: %v\n", err)
		return 1
	}
	if !res.Skipped {
		fmt.Fprintf(stdout, "webhook: POST %s → HTTP %d\n", res.URL, res.StatusCode)
	}
	return 0
}

func importWatch(cfg config.Config, st *state.State, stdout, stderr io.Writer) int {
	found, err := xcsv.WatchCandidates(cfg.XWatchDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(stdout, "x watch dir %s does not exist (skipped)\n", cfg.XWatchDir)
			return 0
		}
		fmt.Fprintf(stderr, "watch dir: %v\n", err)
		return 1
	}
	if len(found) == 0 {
		fmt.Fprintf(stdout, "x: no new CSVs in %s\n", cfg.XWatchDir)
		return 0
	}
	// Re-enter import by constructing a synthetic run: parse here.
	var posts []xcsv.Post
	var daily []xcsv.Daily
	imported := 0
	for _, p := range found {
		res, err := xcsv.ParseFile(p)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", p, err)
			return 1
		}
		if st.SeenX(res.SHA256) {
			continue
		}
		posts = append(posts, res.Posts...)
		daily = append(daily, res.Daily...)
		st.MarkX(state.ImportedFile{
			Path: p, SHA256: res.SHA256,
			ImportedAt: time.Now().UTC().Format(time.RFC3339),
			Rows:       len(res.Posts) + len(res.Daily),
		}, time.Now())
		imported++
		fmt.Fprintf(stdout, "x import %s kind=%s posts=%d daily=%d\n", p, res.Kind, len(res.Posts), len(res.Daily))
	}
	if imported == 0 {
		fmt.Fprintln(stdout, "x: watch directory already imported")
		return 0
	}
	stStore := store.Store{DataDir: cfg.DataDir}
	if err := stStore.WriteX(cfg.XHandle, posts, daily); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func runStatus(gf globalFlags, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "print machine-readable status")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, err := loadCfg(gf)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	st, err := state.Load(cfg.StatePath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	info := map[string]any{
		"version":           config.Version,
		"configDir":         cfg.ConfigDir,
		"dataDir":           cfg.DataDir,
		"youtubeChannelId":  cfg.YouTubeChannelID,
		"xHandle":           cfg.XHandle,
		"xWatchDir":         cfg.XWatchDir,
		"hasClientSecret":   cfg.HasClientSecret(),
		"hasToken":          cfg.HasToken(),
		"webhookConfigured": cfg.WebhookURL != "",
		"youtubeLastEnd":    st.YouTube.LastEnd,
		"youtubeLastRunAt":  st.YouTube.LastRunAt,
		"xLastImportAt":     st.X.LastImportAt,
		"xImportedFiles":    len(st.X.Files),
	}
	if *asJSON {
		b, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, string(b))
		return 0
	}
	fmt.Fprintf(stdout, "socialstats %s\n", config.Version)
	fmt.Fprintf(stdout, "config dir:        %s\n", cfg.ConfigDir)
	fmt.Fprintf(stdout, "data dir:          %s\n", cfg.DataDir)
	fmt.Fprintf(stdout, "YouTube channel:   %s\n", displayChannel(cfg.YouTubeChannelID))
	fmt.Fprintf(stdout, "X handle:          %s\n", displayHandle(cfg.XHandle))
	fmt.Fprintf(stdout, "X watch dir:       %s\n", cfg.XWatchDir)
	fmt.Fprintf(stdout, "OAuth client:      %v\n", cfg.HasClientSecret())
	fmt.Fprintf(stdout, "OAuth token:       %v\n", cfg.HasToken())
	fmt.Fprintf(stdout, "webhook:           %s\n", webhookStatus(cfg))
	fmt.Fprintf(stdout, "YouTube last end:  %s\n", dash(st.YouTube.LastEnd))
	fmt.Fprintf(stdout, "YouTube last run:  %s\n", dash(st.YouTube.LastRunAt))
	fmt.Fprintf(stdout, "X last import:     %s (%d files)\n", dash(st.X.LastImportAt), len(st.X.Files))
	return 0
}

func displayChannel(id string) string {
	if strings.TrimSpace(id) == "" {
		return "auto-detect (channels?mine=true)"
	}
	return id
}

func displayHandle(handle string) string {
	handle = strings.TrimPrefix(strings.TrimSpace(handle), "@")
	if handle == "" {
		return "(not set)"
	}
	return "@" + handle
}

func webhookStatus(cfg config.Config) string {
	if cfg.WebhookURL == "" {
		return "not configured"
	}
	return "configured"
}

func dash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
