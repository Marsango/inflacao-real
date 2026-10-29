# First scraper: next steps

The core is ready to receive and process observations. Build one small, working
collection pipeline before expanding to more shops or regions. Follow
[AGENTS.md](AGENTS.md) and the shared
[ingestion v1 contract](../docs/contracts/ingestion/v1/README.md).

## Implementation checklist

1. **Choose the first shop and collection scope.**
   - Start with one store or delivery region and a limited product set.
   - Identify an available public API, HTML, or embedded structured data source.
     Use a browser only when collection requires it.
   - Register the shop manually in the core database and configure its scraper
     identity, allowed shop ID, and ingestion token outside source control.

2. **Initialize the Python project.**
   - Configure dependencies and tests using the stack selected in `AGENTS.md`:
     Pydantic, HTTPX, and Botasaurus where browser collection is needed.
   - Document installation, configuration, and actual run/test commands.

3. **Implement contract-aligned models.**
   - Model observations, offers, context, and submission envelopes with Pydantic.
   - Validate serialized payloads against the shared schema and examples.
   - Preserve exact decimal prices, package sizes/counts, availability, all
     available price conditions, and store/delivery context. Keep unknowns null.
   - Assign stable observation UUIDs when measurements are collected and stable
     submission UUIDs when batches are assembled. Preserve both during retries.

4. **Implement one shop adapter.**
   - Separate fetching from parsing; parse saved responses without network access.
   - Keep source identifiers stable and record the actual observation time.
   - Add configurable timeouts, conservative request rates, and bounded retries.
   - Report extraction failures and incomplete runs explicitly. Disable result
     caching for fresh collections.

5. **Implement shared batching and durable delivery.**
   - Use one HTTPX client for authenticated submissions to the core.
   - Bound batches by both 100 observations and 1 MiB of encoded request bytes.
   - Persist each immutable batch in a local durable spool before delivery.
     Define its capacity and report exhaustion explicitly.
   - Recover queued batches after restarts and core outages. Retry transient
     failures with bounded backoff, preserving payloads, IDs, and timestamps.
   - Mark batches delivered after an empty `202 Accepted` or `200 OK` response.
     Surface authentication and validation failures for investigation.

6. **Test parsing and delivery.**
   - Save small representative source fixtures and test Brazilian price formats,
     package quantities, conditional offers, unavailable products, and missing
     fields where supported by the shop.
   - Test contract compatibility, count/byte batch limits, spool recovery,
     interrupted delivery, lost-response redelivery, and rejected submissions.
   - Keep routine tests independent of live shops and browsers. Verify live
     collection separately from the intended runtime machine.

## First milestone: two real collection runs

Run the same small collection scope at two different times and verify:

- Products, packages, offers, availability, and location context are correct in
  PostgreSQL, with products and shop listings created or matched automatically.
- The second run produces fresh observation IDs and timestamps, even when prices
  have not changed. Compatible measurements reuse the expected catalog records.
- Interrupted submissions can be delivered again without duplicate history or
  changed payloads and identities.
- Accepted submissions actually reach `processed`. HTTP `202` confirms durable
  queueing; it does not confirm successful background processing. Investigate
  pending retries and failed payloads using core diagnostics.

Database inspection belongs to manual verification and integration tests; the
scraper itself communicates with the core only through its HTTP API. Any test
requiring PostgreSQL must use disposable Testcontainers and automatic cleanup,
never an existing development or production database.

## After the first scraper works

- Add a core history API and a useful web view to complete the first vertical
  slice: collection → ingestion → persisted history → visible price changes.
- Add scheduling once collection and delivery are reliable.
- Implement automated raw-payload cleanup and retention before sustained volume
  makes storage growth an operational concern.
- Track collection coverage and incomplete runs before deriving aggregate metrics.
- Define baskets, weighting, comparison periods, missing-data treatment, and
  methodology before presenting inflation measures or official-index comparisons.

These follow-up capabilities do not block collecting the first observations.
