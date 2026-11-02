# Core platform — Go

## Scope and responsibilities

The core owns ingestion, product identity, historical observations, analytics, persistence, and the HTTP APIs consumed by scrapers and the web UI. Scrapers may run on multiple machines and submit to a centralized core over HTTP.

Start with a modular Go application. Keep ingestion, catalog, history, and analytics logically separate while avoiding unnecessary services or package layers.

## Implementation patterns

- Keep entry points and wiring small. `cmd/core/` initializes the application; `internal/database/` owns PostgreSQL connections and embedded Goose migrations. Organize additional application code by cohesive responsibility under `internal/`.
- Keep HTTP handlers focused on decoding, validation, calling application logic, and encoding responses. Keep calculations independent of HTTP and database details.
- Define small interfaces at the consuming boundary when they support a concrete dependency or useful test seam. Avoid generic repository and service abstractions without a demonstrated need.
- Pass `context.Context` through request-scoped database and network operations, set appropriate timeouts, and propagate cancellation.
- Return explicit errors with useful context. Map them to consistent API responses without exposing credentials or internal details.
- Use structured logs with request or collection identifiers to trace ingestion and processing failures.
- Format Go code with `gofmt` and follow established Go conventions.

## Ingestion and contracts

- Own the versioned schemas and examples in `docs/contracts/`. The initial contract is [ingestion v1](../docs/contracts/ingestion/v1/README.md), with its machine-readable [schema](../docs/contracts/ingestion/v1/schema.json). Follow it when implementing endpoints, persistence, and workers.
- Keep required and nullable fields, monetary and timestamp representations, and retry/idempotency behavior aligned with that payload contract. Enable schema format validation and enforce the documented application-level rules separately.
- Accept bounded batches of observations in a shared format across shops. Keep shop-specific extraction behavior in the scrapers.
- Authenticate scraper submissions and enforce request byte and observation-count limits. Validate the envelope and required structure before accepting a submission; perform normalization and domain processing in the background worker.
- Persist the accepted payload and its queue metadata in PostgreSQL, then return an empty `202 Accepted` response only after the transaction commits. Acceptance means durably queued, not successfully processed.
- Require a stable scraper submission ID and enforce uniqueness on `(scraper_id, submission_id)`. An existing identity must not enqueue again. Submission deduplication is by identity only; no submission fingerprint is retained and changed content under the same identity is ignored, as documented in the ingestion contract.
- Separate source observation time from core receipt time. Define observation uniqueness independently of submission uniqueness so a retried or repackaged batch cannot create duplicate measurements, while legitimate later observations remain distinct.
- Initially accept or reject each submitted batch as a whole and process it atomically. Keep processing state and failure diagnostics internal to the core, with structured logs for investigation.

## Background processing

Use a PostgreSQL ingestion table as the initial durable queue and a goroutine started with the Go application as its worker. A queue library or dedicated broker is not required for this design.

The table should contain the core ID, scraper identity, scraper submission ID, schema version, immutable `jsonb` payload, status, receipt/processing timestamps, attempt count, next-attempt time, and last error. Do not add separate run, collector, or payload-fingerprint columns. Use `pending`, `processed`, and `failed` as the initial states. Store the payload once; do not copy it into a separate job record.

- Start with one worker, a configurable idle polling interval of one minute, and a configurable batch size of 100. These are initial defaults, not throughput targets.
- Check immediately on startup. Process the oldest eligible pending submissions first, with a stable tie-breaker such as ID. After each batch, continue while eligible work remains; wait for the polling interval when no work is ready.
- Keep the loop sequential so timer events cannot launch overlapping processing cycles. Bind it to the application lifecycle and support cancellation and graceful shutdown.
- Treat the batch size as a per-cycle limit. For each payload, select and lock one eligible row with `FOR UPDATE SKIP LOCKED` inside its own transaction. Recheck eligibility under the lock; do not reserve 100 rows in one long transaction.
- Save the normalized observations and mark that payload processed in the same transaction. Commit each payload independently so an unrelated failure does not roll back the whole fetched batch. Row locks coordinate workers if multiple core instances are introduced.
- Keep processing transactions short and free of slow external calls. If work later requires long-running operations, introduce a documented lease and crash-recovery mechanism before moving processing outside the transaction.
- On failure, roll back processing writes, then persist retry metadata in a separate short transaction with concurrency-safe updates. Use bounded retries with backoff, mark exhausted or permanent failures as failed, and continue with other eligible submissions. Failed payloads must remain inspectable and support deliberate replay.
- Assume processing may be attempted more than once. Enforce idempotency through database constraints and transactional writes, not an in-memory processed list.
- Index pending-work selection using queue metadata. Do not index the payload by default. Track backlog size, oldest pending age, processing failures, and retry counts.

## Storage lifecycle

- Use JSONB for incoming payloads and a short replay/debugging window. Use `products` for canonical variants, `shops` for retailers/marketplaces, `shop_products` for source listings with optional canonical matches, `price_observations` for historical measurements, and `ingestion_payloads` for the durable queue. History has one row per observed offer, or a null-price row when offers are empty. Keep observed package sizes and price conditions in history independently of current catalog metadata. See [core/README.md](README.md) for mapping, deduplication, and retention rules.
- Preserve observation provenance and corrections in the normalized model so raw-payload cleanup does not remove the context needed to explain results. Use versioned migrations for database changes.
- Keep ingestion payloads immutable until retention cleanup. JSONB preserves document meaning rather than exact request bytes; if byte-for-byte source preservation becomes a requirement, retain original bodies separately.
- Make successful-payload retention configurable, initially seven days after successful processing. Never expire pending or failed payloads through the successful-payload cleanup policy. Define any separate failed-payload retention explicitly.
- Retain submission and observation identities internally after payload cleanup so retries and rebatching remain deduplicated. Define a deduplication window explicitly before expiring those records. Require scraper-supplied identities on every historical offer; see [core/README.md](README.md).
- Compact unchanged same-day history per scraper, shop listing and shopping context. Compare the complete offer state, package and availability against the latest retained state; preserve actual changes and next-day measurements. Keep every retry identity, original provenance and the latest unchanged confirmation time. See the ingestion contract for the precise policy.
- Measure representative stored payload sizes, observation volume, indexes, backups, and database growth before selecting additional storage infrastructure. Account for cleanup and autovacuum in operating the queue.
- Parquet is a future option for compressed historical analytics and archival, not the active queue format. Add it when measurements justify it; CSV is appropriate for interchange or user exports.
- If Parquet archival is introduced, use typed columns and sensible time-based batches rather than one tiny file per submission. Verify exports and retain provenance and methodology context before removing corresponding database history. Define how historical API queries will access archived data before moving history out of PostgreSQL.
- An archive of normalized observations does not preserve the original submission automatically. If long-term source replay is required, archive source payloads separately in a compressed format and verify their durable storage before cleanup.

## Persistence and measurement

- Apply pending Goose migrations from Go during startup before serving requests or starting workers. SQL migrations live in `internal/database/migrations/` and are embedded in the binary. Use the migration lock to coordinate simultaneous starts, fail startup on migration errors, and never roll back automatically.
- Preserve source observations and provenance. Record corrections explicitly so historical results remain explainable and reproducible.
- Separate source product identity from canonical product identity. Do not merge products based only on similar names.
- The worker automatically creates and matches products using [the documented catalog rules](README.md#automatic-product-matching). Preserve the historical `price_observations.product_id` and match method when a listing changes its current product; use the observation's product for history queries.
- Compare equivalent products and quantities. Treat package-size changes, substitutions, and conditional prices explicitly.
- Distinguish an individual product's price change from an aggregate inflation measure.
- Document basket composition, weighting, time windows, missing-data treatment, and methodology versions for aggregate metrics.
- Do not interpret missing observations or unavailable products as unchanged prices. Make coverage, freshness, and insufficient data visible in API results.

## Web API

Expose product histories, comparisons, and aggregate insights through documented responses. Return the units, periods, coverage, and methodology context required for the UI to explain results. Define consistent filtering, pagination, and errors as endpoints are introduced.

Keep authoritative calculations here so all consumers receive consistent values.

## Verification

Test meaningful domain behavior with explicit inputs and expected results, especially money calculations, quantity normalization, missing data, and date boundaries.

Use PostgreSQL integration tests where constraints, migrations, transactions, or deduplication determine correctness. Cover repeated submission IDs, concurrent worker selection, rollback without partial observations, retry persistence, and recovery after interrupted processing. Verify that payload cleanup preserves submission identities and excludes pending and failed work.

Verify API validation, request limits, acknowledgement only after commit, and payload compatibility at HTTP boundaries. Test polling and backoff with controllable timing rather than minute-long sleeps. Keep pure calculation tests independent of external services.

Run formatting, `go test ./...`, and `go vet ./...` from `core/`. Database integration tests use the shared `testDatabase(t)` helper to start a fresh PostgreSQL Testcontainer, apply migrations, and register pool/container cleanup. Docker must be available. Do not connect tests to an existing development or production database.
