# Scraper reports v1

`POST /api/v1/ingestion/scraper-reports` accepts a completed collection report
using the existing ingestion Bearer token and collector/shop authorization.
Send `Content-Type: application/json`. See [schema](schema.json) and
[example](example.json). This contract does not change observation ingestion.

`scraper_name` is the scraper-supplied retailer display name (Mambo, Festval,
etc.), stored with the run and returned to admins. Identity remains the pair
`(scraper_id, shop_id)`; names are not authorization identifiers.

Times require explicit timezones. Finish cannot precede start. Counts are
nonnegative; errors plus warnings must equal the number of diagnostic entries
plus omitted entries. Each severity count must cover its retained entries.
Reports are limited to 256 KiB, 100 entries, 2,000 characters per message and
100 characters per name/category. The Python client truncates messages and
omits excess entries while retaining complete severity counts. Target/product
context belongs in the message. Do not include secrets, headers, or raw pages.

Return empty `201` after durable commit, or empty `200` for an existing
collector/shop/run ID. Duplicate content is ignored, including after diagnostic
deletion. Run summaries retain deduplication identities indefinitely.
`400` means invalid or oversized report, `401` invalid token, `403` unauthorized
identity, and `503` persistence unavailable. Do not retry permanent rejections
within a run. Pending rejected reports remain available for investigation.

The collector saves exact report bytes in `<spool_path>.diagnostics`, a separate
SQLite queue with the configured spool capacity as its independent payload
budget. Pending reports replay before collection and after saving the new run.
Only an empty 200/201 acknowledges delivery. Failed delivery retains reports
and returns a nonzero collection exit status. A full or unavailable queue fails
explicitly; queued diagnostics are never overwritten. A forced process kill
can prevent a completed report from being created.

## Admin API

Existing admin session/origin/CSRF rules apply. Responses use no-store.

- `GET /api/v1/admin/scrapers`: latest completed run for each collector/shop,
  including `scraper_name`, times, counts and `status`. Authorized registered
  shops without reports appear as `unknown`; otherwise errors take precedence
  over warnings, then healthy. Ordering uses run start time and run ID, so late
  older reports cannot replace newer health.
- `GET /api/v1/admin/scraper-diagnostics?scraper_id=…&shop_id=…`: newest 50 entries,
  including string ID, run ID, severity, category, message and run start time.
  Optional `severity=warning|error` filters results. `before=<last-id>` fetches
  older entries. An empty list ends pagination.
- `DELETE /api/v1/admin/scraper-diagnostics?scraper_id=…&shop_id=…&id=…` deletes
  one entry. Omit `id` to delete all entries for that collector/shop. Returns 204.
  Run summaries and health remain intact. There is no automatic expiry.

Health describes reported collection only: no missed-schedule inference,
heartbeat, core processing status, or alerting. The UI exposes run timestamps
and older retained issues independently of latest-run health.
