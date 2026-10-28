# Changelog

## 2026-10-28

The YouTube channel id and X handle are no longer built in. An empty channel
id is detected from the authorized account (`channels?mine=true`). The X
handle is an optional label. Config file, `SOCIALSTATS_CHANNEL_ID` /
`SOCIALSTATS_X_HANDLE`, and `--channel-id` / `--x-handle` still override.
Home Manager options `youtubeChannelId` and `xHandle` (`nullOr str`, default
`null`) pass those env vars through only when set.

`socialstats auth` and `socialstats fetch` print the first-time Google Cloud
guide when `client_secret.json` is missing. `socialstats setup` (`init`)
prints the same guide, creates the config directory, and writes `config.json`
from flags or terminal prompts. Secrets stay in `$XDG_CONFIG_HOME/socialstats`.

Verification: `go test ./...`, `nix build`.

Affected: `internal/config`, `internal/cli`, `internal/youtube`,
`nix/hm-module.nix`, `README.md`, `config.example.json`, `docs/wishlist.md`.

## 2026-10-28

Initial socialstats CLI: YouTube Analytics + Data API v3 via installed-app
OAuth (read-only), X analytics CSV import, JSON/CSV snapshots under
`$XDG_DATA_HOME/socialstats`, optional webhook POST, Nix flake with
`packages.x86_64-linux.default`, `apps.default`, and
`homeManagerModules.default` (systemd user timer).

Verification: `go test ./...`, `nix build`, `result/bin/socialstats --help`,
`result/bin/socialstats fetch --dry-run`.

Affected: `cmd/socialstats`, `internal/*`, `flake.nix`, `nix/hm-module.nix`,
`README.md`.

## 2026-10-28

X account overview CSVs store dates like `Wed, Oct 28, 2026`. Those strings
were kept as-is, so the summary range was a lexicographic min/max, and
overview totals stayed empty because only post rows were summed. Shares,
unfollows, bookmarks, and the overview column names were not mapped onto
daily rows.

Dates are now `YYYY-MM-DD` in `x/daily.csv` and `x/daily.json`. The summary
range uses those dates, and totals sum impressions, engagements, likes,
bookmarks, shares, new follows, unfollows, replies, reposts, and profile
visits. A file imported before this fix is read once more and merged by day.
A second import of the same file stays skipped.

Verification: `go test ./...`, `nix build`.

Affected: `internal/xcsv`, `internal/store`, `internal/state`, `testdata/x_account_overview.csv`, `README.md`.
