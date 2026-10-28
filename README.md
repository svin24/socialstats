# socialstats

socialstats: a tool for agents that collects your own YouTube and X creator analytics into local JSON/CSV files.

The output files (`latest.json`, the daily and video JSON/CSV files, and the X import files) are meant to be read by AI agents on a schedule. Point an agent at the data directory; it does not need to call YouTube or X itself.

YouTube data comes from the official **YouTube Analytics API** and **YouTube Data API v3** with OAuth 2.0 (installed-app / loopback, read-only scopes). X's own-metrics API is paid, so X data is imported from the official account analytics CSV export. There is no cookie theft and no credential scraping.

Nothing secret or personal is hardcoded. There is no env directory in this repository. The Google OAuth client JSON, refresh token, and optional `config.json` live only in `$XDG_CONFIG_HOME/socialstats` (usually `~/.config/socialstats`).

The YouTube channel id is optional. When `youtubeChannelId` is unset, `fetch` calls `channels?mine=true` and uses the channel on the authorized Google account. Set it in `config.json`, `SOCIALSTATS_CHANNEL_ID`, or `--channel-id` to override that. The X handle is an optional label (`xHandle`, `SOCIALSTATS_X_HANDLE`, or `--x-handle`). Imported CSV rows do not need it.

## First-time setup

1. Open Google Cloud Console (https://console.cloud.google.com/projectcreate) and create a project, e.g. "socialstats".
2. Enable both APIs: YouTube Analytics API (https://console.cloud.google.com/apis/library/youtubeanalytics.googleapis.com) and YouTube Data API v3 (https://console.cloud.google.com/apis/library/youtube.googleapis.com).
3. Set up the OAuth consent screen (https://console.cloud.google.com/auth/overview): External, app name + your email, and under Audience add the Google account that owns the channel as a test user.
4. Under Clients (https://console.cloud.google.com/auth/clients), Create client, type Desktop app, download the JSON.
5. Save it as ~/.config/socialstats/client_secret.json and run `socialstats auth`; sign in with the channel's account, allow read-only access, and click Continue on the "unverified app" warning since it's your own app.

Then: run `socialstats fetch`. Also note: in Testing mode Google expires the token after 7 days; publish the app under Audience to avoid weekly re-auth.

`socialstats auth` and `socialstats fetch` print this guide, with the resolved client path, when `client_secret.json` is missing. `socialstats setup` (alias `init`) prints it, creates the config directory (mode 0700), and writes `config.json` from `--channel-id`, `--x-handle`, `--watch-dir`, or from prompts when stdin is a terminal. Leave the channel id empty to auto-detect.

If `$XDG_CONFIG_HOME` is set, the client file is `$XDG_CONFIG_HOME/socialstats/client_secret.json`. The commands print that resolved path.

Scopes the tool requests (add these on the consent screen if Google asks):

- `https://www.googleapis.com/auth/yt-analytics.readonly`
- `https://www.googleapis.com/auth/youtube.readonly`

Do not add `yt-analytics-monetary.readonly`. This tool does not request estimated revenue.

## Commands

```
socialstats setup      # print the guide, create the config dir, optional config.json
socialstats auth       # one-time Google OAuth loopback; stores a refresh token
socialstats fetch      # pull YouTube Analytics; also import new X CSVs
socialstats import-x   # normalize an X analytics CSV (or watch ~/Downloads)
socialstats status     # paths, auth present?, last run
```

Useful flags:

- `--dry-run` — print the plan; do not call Google, write files, or POST
- `--since YYYY-MM-DD` / `--until YYYY-MM-DD` / `--days N`
- `--channel-id` — override auto-detect
- `--x-handle` — optional label stored with X imports
- `--config-dir` / `--data-dir`
- `--webhook-url` / `--webhook-url-file`
- `import-x --watch-dir DIR` or `import-x /path/to/export.csv`

Default YouTube window is the last 28 days ending yesterday. Later runs are incremental from the last end date with a 2-day overlap (YouTube revises recent numbers).

## Where files go

| Kind | Default path |
| --- | --- |
| OAuth client JSON | `~/.config/socialstats/client_secret.json` |
| Refresh token | `~/.config/socialstats/token.json` (mode 0600) |
| Optional config | `~/.config/socialstats/config.json` |
| Run state | `~/.config/socialstats/state.json` |
| Normalized data | `~/.local/share/socialstats/` |

Override with `--config-dir`, `--data-dir`, or `SOCIALSTATS_CONFIG_DIR` / `SOCIALSTATS_DATA_DIR`.

Data layout:

```
~/.local/share/socialstats/
  latest.json                 # combined summary (also POSTed if a webhook is set)
  latest.csv
  youtube/
    channel.json
    daily.{json,csv}          # deduped by day
    videos.{json,csv}         # deduped by video id
    traffic_sources.{json,csv}
    cards.{json,csv}
    impressions.{json,csv}    # only if the API accepts the query
    snapshots/YYYYMMDDThhmmssZ.json
  x/
    posts.{json,csv}          # deduped by post id
    daily.{json,csv}          # deduped by date
    snapshots/…
```

`config.example.json` shows the shape with placeholders (`example_handle`, a fake channel id). Copy it to the config directory only if you want overrides. Omit `youtubeChannelId` to auto-detect. Do not put client secrets or tokens in it, and do not commit a filled-in copy.

## What is pulled from YouTube

- Channel snapshot (title, subscriber / view / video counts) via Data API. With no channel id configured, this is `channels?mine=true`.
- Channel totals and **daily** metrics: `views`, `estimatedMinutesWatched`, `averageViewDuration`, `subscribersGained`, `subscribersLost`, `likes`, `comments`, `shares`
- Per-video metrics (top 200 by views, plus more uploads up to `--max-videos`)
- Traffic sources (`day,insightTrafficSourceType`)
- Card impressions / clicks / CTR (these **are** in the Analytics API)
- Thumbnail impressions / CTR **if** `reports.query` accepts them. Google currently documents `videoThumbnailImpressions` mainly on the bulk Reporting API; the targeted query often returns "query not supported". socialstats tries, then tries the older `impressions` / `impressionClickThroughRate` names, then records a note and continues.

`socialstats fetch --dry-run` prints the plan without credentials. A real fetch needs the client JSON and a refresh token from `socialstats auth`.

## X analytics CSV export

X does not offer a reasonable, non-fragile official way to download own-account analytics without a paid API. Automated cookie/session scraping is out of scope. Export the CSV yourself:

1. In a desktop browser, sign in as the account you want to track.
2. Open [x.com/i/account_analytics](https://x.com/i/account_analytics) (or [analytics.x.com](https://analytics.x.com) → **Posts**).
3. Set the date range. Post exports are often capped (about 30 days and 3,000 posts). The account overview export can cover a longer window.
4. Use **Export** to download a CSV. Timestamps are UTC.
5. Either:
   - Drop the file in `~/Downloads` with a name like `account_overview*.csv`, `account_analytics*.csv`, `tweet_activity*.csv`, or `post_activity*.csv`, then run `socialstats fetch` / `socialstats import-x`
   - Or pass the path: `socialstats import-x ~/Downloads/account_overview_analytics.csv`

`import-x` accepts both **post-level** Tweet/Post Activity CSVs and **daily account** CSVs, including the account overview header (`Date`, `Impressions`, `Likes`, `Engagements`, `Bookmarks`, `Shares`, `New follows`, `Unfollows`, `Replies`, `Reposts`, `Profile visits`, `Create Post`, `Video views`, `Media views`). Dates such as `Wed, Oct 28, 2026` are stored as `YYYY-MM-DD`. The summary range is the earliest and latest day, and its totals sum the daily account metrics (impressions, engagements, likes, bookmarks, shares, new follows, unfollows, replies, reposts, profile visits). Headers are matched fuzzily (Tweet vs Post, comma vs semicolon, UTF-8 BOM, quoted commas/newlines). Already-imported files are skipped by SHA-256. A parser change re-reads a file once and merges by day, so corrected dates replace the old rows. The next run of the same file is skipped again.

The handle stored next to those rows is only a label. Leave it unset, or set `xHandle` in `config.json`.

## Optional webhook

Webhook delivery is optional and off unless you set a URL. If `SOCIALSTATS_WEBHOOK_URL` or `webhookUrl` / `webhookUrlFile` is set, `fetch` and `import-x` POST `latest.json` as `Content-Type: application/json`. The payload is the public summary (counts, totals, top videos). It does not include tokens or the OAuth client. See `docs/wishlist.md` for why this stays optional.

```bash
export SOCIALSTATS_WEBHOOK_URL="https://example.invalid/hooks/socialstats"
# or: echo 'https://…' > ~/.config/socialstats/webhook-url
socialstats fetch --webhook-url-file ~/.config/socialstats/webhook-url
```

## Nix build and Home Manager

```bash
nix build                  # packages.x86_64-linux.default
nix run . -- --help
nix run . -- fetch --dry-run
```

The flake exposes `apps.x86_64-linux.default` and `homeManagerModules.default`.

Import the module from your own Nix configuration. This repository does not ship a personal env file.

```nix
{
  inputs.socialstats.url = "git+file:///path/to/socialstats";
}
```

```nix
{ inputs, ... }:
{
  imports = [ inputs.socialstats.homeManagerModules.default ];

  programs.socialstats = {
    enable = true;
    interval = "daily"; # systemd OnCalendar; or "*-*-* 07:00:00"
    # youtubeChannelId = "UCexample000000000000000"; # null = channels?mine=true
    # xHandle = "example_handle";                    # null = no label
    # configDir = "/home/user/.config/socialstats";
    # webhookUrlFile = /home/user/.config/socialstats/webhook-url;
  };
}
```

`home-manager switch` installs the binary and a **systemd user timer** that runs `socialstats fetch`. The timer does not run `auth`. Do that once interactively so a refresh token exists before the first scheduled fetch.

Module options: `enable`, `package`, `interval`, `configDir`, `youtubeChannelId`, `xHandle`, `webhookUrl`, `webhookUrlFile`. `youtubeChannelId` and `xHandle` are `nullOr str`, default `null`, and are passed through as `SOCIALSTATS_CHANNEL_ID` and `SOCIALSTATS_X_HANDLE` only when set.

Install the built binary without Home Manager:

```bash
nix build
ln -sfn "$(readlink -f result)/bin/socialstats" ~/.local/bin/socialstats
socialstats --help
```

## Development

```bash
nix develop
go test ./...
```

Toolchain is Go. Dependencies: `golang.org/x/oauth2` only. Google API calls are plain HTTPS; tests use `httptest` and never hit the network.

## What this tool will not do

- Create or push a GitHub repo
- Run OAuth for you in an unattended session
- Scrape X with cookies
- Store secrets or personal ids in git
- Request monetary YouTube scopes
