# 2026-10-28 shareable defaults and first-run setup

## Goal

Remove built-in personal defaults so the tool can be shared. The YouTube
channel id is detected from the authorized account when unset. The X handle
is an optional label. Home Manager can pass either value through the
environment. First-run `auth` / `fetch` prints the Google Cloud guide when
the OAuth client JSON is missing.

## Constraints

- Edits stayed in this repository, plus one XDG config file outside git.
- Did not edit `~/dev/nix`, `~/work`, or any GitHub remote.
- Did not run a live OAuth flow.

## Verification

| Command | Exit | Result |
| --- | --- | --- |
| `gofmt -w` on edited Go files, then `go test ./...` | 0 | all packages ok |
| `nix build -L` (checkPhase runs `go test`) | 0 | `socialstats-0.1.0` |
| `nix-instantiate --parse nix/hm-module.nix` | 0 | syntax ok |
| `result/bin/socialstats version` | 0 | `0.1.0` |
| `result/bin/socialstats fetch --dry-run` with an empty config dir | 0 | channel line is auto-detect via `channels?mine=true`; no files written |
| `result/bin/socialstats auth` with no client JSON | 1 | guide printed, step 5 shows the resolved `client_secret.json` path |
| `result/bin/socialstats status` with an empty config dir | 0 | channel auto-detect, X handle not set |
| `strings result/bin/socialstats` scanned for the old personal ids | 0 | no matches |

Not verified: a live `channels?mine=true` call. The HTTP behavior is covered
by `TestFetchDetectsChannelViaMine` against `httptest`.

The owner's `config.json` was created under `$XDG_CONFIG_HOME/socialstats`
(or `~/.config/socialstats`) only because that file was absent. It is not
in this repository.
