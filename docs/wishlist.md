# Wishlist

Ideas that are not part of the current tool. The files under the data directory are the interface agents read.

- Optional webhook push of latest.json after each run (deferred; the tool is primarily consumed by agents reading local files, and an official X MCP/API for own analytics would make the CSV step and webhook unnecessary). Webhook code already exists (`internal/webhook`, `fetch` / `import-x`) but is optional: nothing is sent unless `SOCIALSTATS_WEBHOOK_URL`, `webhookUrl`, or `webhookUrlFile` is set.
