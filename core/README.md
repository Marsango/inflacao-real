# Core

Go module for the core platform. Startup connects to PostgreSQL, applies pending
Goose migrations, starts the ingestion worker, and serves a Gin HTTP ingestion API. SQL files are embedded
in the binary; a Goose CLI and a migrations directory on the deployed filesystem
are not required.

## Run

Start PostgreSQL using the root Compose file, then run from `core/`:

```sh
go run ./cmd/core -env-file ../.env
```

Use Go 1.26 or newer. The optional dotenv file uses the existing Compose settings;
already-exported environment variables take precedence. Alternatively, supply
`DATABASE_URL` in the environment. It takes precedence over `POSTGRES_*` settings.

| Variable | Default |
| --- | --- |
| `POSTGRES_HOST` | `127.0.0.1` |
| `POSTGRES_PORT` | `5432` |
| `POSTGRES_DB` | `inflacao_real` |
| `POSTGRES_USER` | `inflacao_real` |
| `POSTGRES_PASSWORD` | Required unless using `DATABASE_URL` |
| `POSTGRES_SSLMODE` | `disable` for local development |
| `HTTP_ADDR` | `127.0.0.1:8080` |
| `INGESTION_TOKEN` | Required secret Bearer token, without whitespace |
| `INGESTION_SCRAPER_ID` | Required scraper identity bound to the token |
| `INGESTION_SHOP_IDS` | Required comma-separated list of allowed shop IDs |
| `INGESTION_POLL_INTERVAL` | `1m` |
| `INGESTION_BATCH_SIZE` | `100` payloads per cycle |
| `INGESTION_MAX_ATTEMPTS` | `5`, including the initial attempt |
| `INGESTION_RETRY_BASE` | `1m` |
| `INGESTION_RETRY_MAX` | `1h` |

Startup has a one-minute connection/migration timeout and fails if either step
fails. Goose tracks applied versions and uses a PostgreSQL advisory lock to
serialize migrations across simultaneous core starts. Repeated starts apply only
new migrations. The process serves requests until SIGINT/SIGTERM, then drains
active requests for up to ten seconds before closing the database. Database
operations have a ten-second request timeout and inherit request cancellation.
Shutdown cancels and joins the worker before closing the database; unfinished
processing rolls back and remains pending for the next startup.

## Submit a payload

Configure the ingestion settings above in `.env` (see `.env.example`). The initial
configuration binds one token to one scraper identity and its allowed shops.
Use HTTPS when exposing the API outside local development.

Send the complete [v1 envelope](../docs/contracts/ingestion/v1/README.md) as the
request body, without an extra `ingestion_payload` wrapper. For example, with
`INGESTION_TOKEN` exported in your shell and the example scraper/shop authorized,
run from the repository root:

```sh
curl -i http://127.0.0.1:8080/api/v1/ingestion/submissions \
  -H "Authorization: Bearer $INGESTION_TOKEN" \
  -H 'Content-Type: application/json' \
  --data-binary @docs/contracts/ingestion/v1/examples/submission.json
```

The endpoint authenticates the caller, validates the shared schema with format
checks, authorizes the submitted identities, and stores the complete JSONB body.
Requests are limited to 1 MiB and 1–100 observations. Duplicate JSON keys,
observation IDs within a batch, invalid Unicode/URLs, and unknown fields are rejected.

New submissions receive an empty `202 Accepted` only after the insert commits.
PostgreSQL supplies `status = 'pending'`, `attempts = 0`, `received_at` and
`next_attempt_at`; processing/failure/cleanup metadata starts null. A repeated
`(scraper_id, submission_id)` receives an empty `200 OK` without replacing the
payload or resetting state. Deduplication is by identity only, including after
raw-payload cleanup; changed content under the same ID is not detected. Errors
use `{"error":"<code>"}` with the statuses documented in the contract.

The endpoint only queues submissions. The background worker creates or matches
products, upserts listings, and writes price observations as described below.
Raw-payload cleanup is not implemented.

The runtime schema is embedded in the binary. After editing the shared schema,
run `go generate ./internal/ingestion/document` from `core/`; a test checks that the copy
matches the shared contract.

## Background worker

Register each allowed shop in `shops` before processing its submissions; the
worker does not invent retailer names or website URLs. An allowed scraper shop
ID is authorization, not automatic catalog registration. Missing shops cause
retryable failures.

One goroutine checks immediately on startup, selects the oldest eligible pending
payload with `FOR UPDATE SKIP LOCKED`, and processes it within a ten-second
transaction. Each payload commits independently: identity records, product creation/matching, listing
changes, historical offers, and the `processed` transition succeed together.
Cycles drain ready work in configurable batches and wait only when no work is
ready or an infrastructure error prevents progress. No source URLs are fetched.

The worker revalidates stored v1 envelopes, checks GTIN digits and reference-price
relationships, and keeps money and quantities as exact decimals. It creates or
matches canonical products and upserts shop-scoped listings automatically. Older
observations do not overwrite newer listing metadata. It saves every offer in
array order; an empty offers array produces one row with null offer fields.

Observation identity is `(scraper_id, observation_id)`. A SHA-256 fingerprint over
the contract's JCS observation document includes the shop and complete observation.
Identical observations in a different submission are no-ops; conflicting content
fails the whole payload. Run and collector provenance refer to the first accepted
measurement and are retained with its identity. Money remains strings when
fingerprinting. The canonicalizer supports only the validated v1 schema, whose
JSON numbers are bounded integers and whose object keys are fixed ASCII names.

On failure all processing writes roll back. A separate, conditional update records
the attempt and diagnostics without overwriting a competing worker's newer state.
Transient errors use exponential backoff (`1m`, `2m`, `4m`, ... capped at `1h`
by default). Invalid payloads, invalid GTINs, invalid reference prices, and identity
conflicts fail permanently on the first attempt. Other errors stop after five
attempts by default. Pending and failed payloads retain their original JSON.
Structured logs report payload/submission IDs, failures, backlog size, oldest
pending age, failed count, and the number of attempts beyond the initial attempts.

After investigating and fixing the cause, deliberately replay a failed payload
by resetting its queue metadata, leaving its body and identity unchanged:

```sql
UPDATE ingestion_payloads
SET status = 'pending', failed_at = NULL, attempts = 0,
    next_attempt_at = now(), last_error = NULL
WHERE id = <payload_id> AND status = 'failed';
```

Correcting conflicting observation data requires a future explicit correction
workflow; replay does not authorize replacing an existing measurement.

## Automatic product matching

### Shared monitoring scope

`GET /api/v1/monitoring-targets` supplies all scrapers with the enabled,
brand-independent discovery targets and deterministic inclusion/exclusion rules.
It requires the ingestion Bearer token. See the
[monitoring v1 contract](../docs/contracts/monitoring/v1/README.md) for the response,
matching semantics, examples, errors and coverage limits.

Migration 00006 registers 16 targets: the DIEESE basket's 13 food groups with
regional flour/sugar variants, plus the previously monitored pasta. See the
[basket scope and research sources](../docs/contracts/monitoring/v1/basic-food-basket.md).
Migration 00007 adds [21 everyday foods](../docs/contracts/monitoring/v1/everyday-foods.md).
Migration 00008 adds [20 foods and eight household essentials](../docs/contracts/monitoring/v1/foods-and-household.md),
migration 00009 adds [12 family essentials](../docs/contracts/monitoring/v1/family-essentials.md),
and migration 00010 adds [19 snacks, sweets and convenience foods](../docs/contracts/monitoring/v1/snacks-and-convenience.md),
for **96 targets** on a fresh database. Restart the updated core to apply pending
forward migrations; scrapers receive the additions at their next run. Update and
restart running scrapers once for the diaper/bag package-parsing guard described
in the family essentials guide.
Add or refine targets through new SQL migrations; scrapers fetch the scope at
the start of every run. New targets require no scraper code or product-category
configuration changes with the default configuration. Muffato and Festval search
the returned queries; Angeloni discovers its live leaf categories automatically
and filters the catalog using the same core rules. An older explicit Angeloni
`catalog_category_ids` list must be removed once to enable automatic discovery.
Source changes and unsupported price formats can still require adapter work.
The worker also applies these rules to source names and associates discovered
canonical products through `product_monitoring_targets`, in the same transaction
as their observations. This groups products without changing identity matching:
brands, variants and sizes stay separate. Existing products are associated as
fresh observations arrive; no initial backfill or aggregate comparison is added.
Associations are additive catalog metadata, not historical scope snapshots.

### Canonical product identity

Only shops need manual registration. Each new observation receives a product and
a shop/SKU listing automatically, within the payload's processing transaction.
Matching follows this order:

1. Find one compatible product with the same validated GTIN. Compare zero-padded
   14-digit GTINs, following [GS1's representation guidance](https://support.gs1.org/support/solutions/articles/43000734355-what-is-the-required-format-of-gtin-in-gs1-edi-standards-).
   Conflicting known brands, package sizes, or component counts prevent a match.
2. Reuse the existing shop/SKU product if the brand, barcode and package do not
   contradict it, and either the barcode agrees or the source name is equivalent
   to that listing's previous name.
3. Find exactly one product with an equivalent name, the same normalized brand,
   and the same complete package. Known conflicting GTINs prevent a match.
4. Otherwise create a separate product from the source attributes, preserving
   nulls for unknown values. Later observations can fill unknown attributes.

Name comparison ignores case, accents, punctuation, and token order. Explicit
measures are normalized exactly: `Café Torrado 500g` and `TORRADO CAFE 0,5 kg`
have equivalent names. Structured packages compare grams/kilograms and
milliliters/liters using exact decimal arithmetic, retaining the component count:
six 350 ml cans do not match one 2.1 l bottle. Other name words and numbers are
retained, so `integral`, `desnatado`, `zero`, flavor words, percentages, and size
differences are not discarded.

This deliberately avoids a fuzzy similarity threshold. A name alone cannot
establish product identity. Missing brand/package data without a GTIN, extra or
different name words, typos, and ambiguous candidates can produce separate
products. This favors keeping uncertain measurements apart over mixing variants
in an inflation calculation. A same-shop listing can still reuse its own product
when its identity remains compatible.

The initial implementation uses a transaction-scoped PostgreSQL advisory lock
around catalog decisions, so concurrent workers cannot both create a product
after seeing no candidate. This serializes catalog writes across core instances;
the worker still locks and commits each payload independently.

Each history row stores the product selected at collection processing time and
its matching method (`gtin_v1`, `attributes_v1`, `listing_v1`, or `created_v1`).
If a shop reuses a SKU for another size or variant, its current listing can switch
products while earlier history retains its original product. Product history
queries must use `price_observations.product_id`, rather than following today's
listing assignment. Migration 00004 leaves these fields null for old history:
it does not infer past identities from current catalog metadata.

## Tables

- `ingestion_payloads`: original JSONB submission and its queue metadata, source
  identity, retry timing, and internal processing state. Run and collector metadata
  are not separate columns; any such fields in the submission remain in its JSON.
- `shops`: retailers and marketplaces. Its text ID is the contract's `shop_id`;
  name and website URL describe the source, not a physical branch.
- `products`: canonical product variants, with name, brand, optional GTIN, and
  package quantity/unit/count. Different package sizes are different variants.
- `monitoring_targets`: enabled collection groups and their shared discovery/name
  rules, seeded by migrations 00006–00010 and served to all shop adapters.
- `product_monitoring_targets`: associations between distinct canonical variants
  and their monitoring groups; membership does not imply price equivalence.
- `shop_products`: a shop's listing, unique by `(shop_id, source_product_id)`,
  with its source name, brand, GTIN, customer-facing URL, and current product.
  `product_id` remains nullable for legacy/manual rows; the worker supplies it.
- `price_observations`: historical offers referencing a listing and ingestion
  payload, with source URL, timestamps, observed package
  size, collection method, seller/location context (including branch/store ID),
  availability, exact prices, all price conditions including coupon requirements,
  and the product/matching method selected for that historical observation.
- `observation_identities`: source measurement IDs and fingerprints, the first
  payload reference, and run/collector provenance. These survive raw-payload cleanup.

Goose also maintains its own `goose_db_version` bookkeeping table.

```text
products ← shop_products → shops
                 ↑
         price_observations → ingestion_payloads
```

Multiple shops' listings can reference the same product. Automatic matching uses
the rules above, with brand and package evidence alongside name comparisons. Listing
metadata describes its current presentation; historical prices, observed package
sizes, conditions, and collection provenance remain on the observations. A
listing's current canonical match does not prove every historical package is
equivalent: compare the observed package sizes and reverify the match if the
source product changes. Seller IDs remain measurement fields until an integration
requires a separate seller table.

A parsed row represents **one offer within one observation**, with `offer_index`
matching its position in the source array. `offer_count` records the array length.
An observation with no offers has one row with index/count zero and all offer
fields null. This preserves unavailable or unknown-price observations without
creating a fake zero price. Payloads, products, listings, and observations have
`bigint GENERATED ALWAYS AS IDENTITY` primary keys and matching numeric foreign
keys. Scraper-generated `submission_id` remains a UUID;
`source_product_id` and the shop's text ID remain source identifiers.

Foreign keys require existing shops, matched products, listings, and payloads,
and prevent deleting referenced records. The worker must verify that a listing's
shop matches the ingestion payload before writing; that cross-table identity
check is an application responsibility. Scraper identity comes from the payload. Shop identity
is reached through the listing rather than repeated in every historical row.
After successful processing and the retention period, clear `payload` and set
`payload_removed_at` together, retaining submission IDs for deduplication.
Queue constraints prevent clearing pending or failed payloads.

The worker inserts all results and marks the payload processed in one transaction.
Submission deduplication remains enforced by `(scraper_id, submission_id)` on
`ingestion_payloads`. There is no stored payload fingerprint, so an existing
submission ID is recognized without detecting changed content under that ID,
as documented in the wire contract. Observation identities are retained separately
and offer rows have a unique `(scraper_id, observation_id, offer_index)` constraint.
Migration 00003 adds these guarantees and provenance fields. Migration 00005
requires both scraper and observation IDs on every historical offer. Observation
IDs come from the scraper and have no database-generated default.

Money and quantities use exact PostgreSQL `numeric` values with explicit range and
scale checks, avoiding implicit rounding from fixed-scale column types. Full JSON
Schema validation, GTIN check digits, and complete offer groups
remain application responsibilities. Product names and customer-facing listing
URLs are stored in the catalog rather than repeated in each historical offer.
Indexes support listing lookup, product-to-listing lookup, listing history, and
payload references; there is no speculative GTIN history index.

## Migrations

Add sequential SQL files under `internal/database/migrations/`, with Goose `Up` and
`Down` sections. Keep applied migrations unchanged; add a new migration for changes.
Startup applies only `Up` migrations. Rollbacks are available through the Goose Go
provider for controlled maintenance and integration tests; startup never rolls back.

Migrations 00001 and 00002 were refactored in place during development to use numeric
internal IDs and introduce the catalog and price history. A database that already
applied earlier versions will not pick up those rewrites. Recreate a disposable
development database, or use a separate forward migration if existing data must
be preserved.

## Verification

```sh
go test ./...
go vet ./...
```

Docker must be running and accessible to your user. Each database integration test
calls the shared `testDatabase(t)` helper, which starts a fresh `postgres:18.6`
Testcontainer on a dynamically assigned port, opens a connection pool, and applies
the embedded migrations. `t.Cleanup` closes the pool and terminates/removes the
container even when a test fails. No `.env`, running Compose database, or
`TEST_DATABASE_URL` is needed. Testcontainers may download container images on the
first run. Tests fail if Docker or container startup is unavailable.

Coverage includes migration up/repeated up/down/up, unmatched listings, cross-shop
matching, listing uniqueness, history preservation, price precision, foreign keys,
missing prices, payload retention, concurrent ingestion retries, queue defaults,
and acknowledgement only after a successful commit.
Worker coverage includes contract fingerprints, GTIN/reference-price validation,
exact prices, unavailable products, provenance, cross-submission deduplication
after payload cleanup, conflict rollback, commit-time failures, bounded retries,
deliberate replay, concurrent row selection, cancellation and restart recovery.
Catalog tests cover name normalization, exact unit equivalence, GTIN matching,
automatic creation, unknown attributes, ambiguous candidates, variant separation,
historical product assignments, and concurrent product creation.
Polling-loop and backoff unit tests use controlled work/wait functions without
minute-long sleeps.

## Web dashboard API

The core exposes public read-only `GET /api/v1/products` (cursor-paginated canonical
variants) and `GET /api/v1/history?product=global&start=YYYY-MM-DD&end=YYYY-MM-DD`
(use a canonical product ID for individual history). See the [versioned web API
contract](../docs/contracts/web/v1/README.md) for response fields, units, date
boundaries, errors and `matched-equivalence-v3` methodology. The browser never needs a
scraper token; ingestion and monitoring routes retain their authentication.

Daily prices use a fixed set of listing/seller/location/channel contexts and
preserve gaps. Known package sizes must be equivalent; explicit per-kg/L/unit
prices are supported when the canonical package is unknown and the observed basis
is unambiguous. Known restricted offers are excluded. Advertised base prices with
unreported conditions are explicitly flagged and never substituted for known
unrestricted prices in a context that has them. The overview equally weights
product price relatives on complete days, without changing the panel when data is
missing. It is an exploratory sample index, not a national inflation measure.

Money averages and relative changes use exact rational arithmetic; HTTP responses
round only at the centavo/basis-point boundary. Coverage, actual comparison dates,
source observation freshness and methodology accompany every response. A strict
fixed panel can have no complete days; individual histories remain available.

For local development, run the core on its default port 8080 and `npm run dev`
from `web/`. Vite forwards `/api` to the core. No frontend deployment is configured.
Tests cover the complete ingest/worker/database/read path using the shared
`internal/testdatabase.New` helper with fresh migrated PostgreSQL Testcontainers.
The existing database tests keep their `testDatabase(t)` wrapper around that helper.

### Reversible product equivalence

Strict ingestion identity is unchanged. Migration 00012 adds separate equivalence
membership, exclusions, catalog revisions, audit and owner sessions. Public search
and histories (including global calculations and legacy member IDs) resolve current
representatives before aggregation. Offer contexts and historical assignments
remain immutable. The default representative is the lowest member ID; the owner
can select another. Group changes serialize with ingestion and stale reviews fail.

The [admin API and rule documentation](../docs/contracts/web/v1/admin.md) describes
Portuguese review, exact/fuzzy branded suggestions, conservative `commodities_v1`
automation, reversals and authentication. Generic grouping starts disabled: preview
and explicitly activate it from `/admin` after backup and source review. The
worker applies enabled rules within each ingestion transaction. No scraper contract
or package-to-kg conversion changes are introduced.
