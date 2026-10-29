# Super Muffato collector

The first adapter discovers products from **Curitiba – Champagnat** using
Super Muffato's public VTEX product API. The core supplies brand-independent
monitoring targets through its authenticated scope endpoint. Migration 00006
seeds 16 targets covering the DIEESE basket's 13 food groups, regional flour/sugar
variants, and pasta. See the [basket scope and sources](../docs/contracts/monitoring/v1/basic-food-basket.md).
Variable-weight meat, produce and bakery items still require adapter support;
the seed does not make their collection complete. Matching brands, variants and
package sizes stay separate products.
These groups are not a statistically representative inflation basket.

## Setup

Run from `scrapers/`, with Python 3.12 or newer:

```sh
python3 -m venv .venv
.venv/bin/pip install --no-cache-dir -e '.[dev]'
cp muffato.example.toml config.toml
```

This makes the Python package and `inflacao-scraper` command available inside
the local virtual environment. Editable mode (`-e`) uses source changes directly;
`[dev]` adds pytest and Ruff. No service, scheduler, browser, or system-wide
installation is needed. Database access is not part of scraper operation or tests.

`config.toml` and `data/` are ignored by Git. Set a unique `scraper_id` for each
independent installation. Keep the ingestion token only in the environment.

## Collection

```sh
.venv/bin/inflacao-scraper --config config.toml discover --query arroz --limit 50
# Start the core and export its matching INGESTION_TOKEN before collection.
.venv/bin/inflacao-scraper --config config.toml collect
.venv/bin/inflacao-scraper --config config.toml status
```

Discovery prints SKU IDs, names, package sizes, availability, and prices without
queueing measurements. It is an ad hoc diagnostic command, independent of the core.

`collect` authenticates to `GET /api/v1/monitoring-targets` once per run, then
searches Muffato for each target's phrases. It follows pagination, filters each
SKU using the central required/excluded name terms, and collects every matching
brand and individually sold package size. A SKU found by several queries or
targets is measured once per run. Brand and variant comparisons remain the core's
responsibility. See the [monitoring v1 contract](../docs/contracts/monitoring/v1/README.md).

Each page's validated observations are checkpointed in `data/spool.sqlite3`
before requesting another page. Collection does not submit observations to the core.
Every new measurement receives a new observation ID and UTC timestamp, even if
the price is unchanged. Run and submission IDs are printed for diagnosis. An
incomplete run exits nonzero and retains already queued observations. Restarting
collection creates a new run; use `deliver` to resume existing deliveries.

The core seeds the initial scope through migration 00006; add/refine targets with
forward migrations. Remove the old `sku_ids` field from existing local TOML files.
`collect` now needs `INGESTION_TOKEN` and a reachable core. It never falls back to
a local product list if scope retrieval or validation fails. Credentials are sent
only to the configured core, never to Muffato; redirects are not followed.

The scraper reports unique matching listings per target. Missing targets, parser
errors, failed pages and repeated pagination mark the run incomplete. Searches
use 50 products per page and follow the source `resources` total when available;
otherwise a short/empty page ends the search. `max_search_pages` defaults to 50 per
query. Reaching that bound while more results may remain is an explicit failure.
An empty enabled scope is a successful no-op. Name filters are conservative:
missing type/processing details may exclude otherwise relevant products. Exhausting
search results does not prove complete retailer coverage.

Requests are synchronous, paced at one per second by default, with explicit
timeouts and at most three attempts for network failures, HTTP 429, and transient
500/502/503/504 responses. Retry-After is honored within the retry-time budget;
longer waits leave work for a later invocation. Redirects are not followed.
No collection-result cache is used. Requests ask for revalidation and use unique
query keys; responses with a positive HTTP Age are rejected as fresh measurements.
Prices can still reflect the shop's own upstream updates and indexing delays.

### Branch and delivery context

On 2026-10-29, the retailer's public
[store-selector script](https://www.supermuffato.com.br/_v/public/assets/v1/published/muffatosupermercados.store-theme@0.0.190/public/react/SelectorSalesChannel.min.js)
mapped `sale_channel:8` to **Curitiba - Champagnat**. Its selection uses the `sc`
URL parameter. The default channel `1` maps to Campo Mourão. Every collection
request explicitly includes `sc=8`; the configuration must retain this mapping
when attaching Curitiba context.

`context.store_id` is `sales-channel:8`, explicitly identifying the storefront
branch selection, not an internal warehouse ID. City/state are Curitiba/PR;
seller ID is `1`. This is an **online branch catalog**: no delivery CEP has been
selected or verified, so `postal_code` remains null and `channel` is `online`.
It does not establish delivery coverage or the final checkout price for an address.

The source follows the [VTEX Search API](https://developers.vtex.com/docs/api-reference/search-api):

```text
GET /api/catalog_system/pub/products/search?fq=skuId:25978&sc=8&_from=0&_to=49
```

HTTP 206 is a normal paginated response. Diagnostic/live checks can fetch an exact
SKU as above; normal collection uses `ft=<target search query>` with `_from`/`_to`
pagination. It examines every returned SKU and selects the configured seller.

### Parsing boundaries

Fetching, pure parsing, contract models, batching, and delivery are separate modules.
JSON prices are decoded as Decimal, never float, then serialized as exact two-place
money strings. Package sizes come only from unambiguous terminal measurements in
names; unknown or ambiguous multipacks remain null. This first adapter supports
individually sold packaged goods (`measurementUnit=un`, `unitMultiplier=1`),
not variable-weight items or kits. Invalid GTINs and malformed observations are reported.

Availability and displayed prices are independent. Missing prices stay unknown;
unavailable zero-price placeholders produce an empty offers array. Promotion
structures and payment alternatives are retained, including full installment
totals. Identical payment terms across card brands share an offer with the brands
listed in its description. Membership, coupon, and minimum-quantity requirements
remain null when the source does not establish them; promotion wording stays in
the description. Offers exceeding contract limits fail validation rather than
being silently truncated.

## Delivery to the core

Configure the running core with matching `INGESTION_SCRAPER_ID` and
`INGESTION_SHOP_IDS=super-muffato`. An administrator must register the shop before
the worker processes observations; the scraper itself never accesses PostgreSQL:

```sql
INSERT INTO shops (id, name, website_url)
VALUES ('super-muffato', 'Super Muffato', 'https://www.supermuffato.com.br')
ON CONFLICT (id) DO NOTHING;
```

Use the existing core setup instructions in [core/README.md](../core/README.md).
With the matching token exported as `INGESTION_TOKEN`, run:

```sh
.venv/bin/inflacao-scraper --config config.toml deliver
```

Delivery reuses the exact persisted UTF-8 body after retries and restarts. Batches
are limited to 100 observations and 1 MiB, including their envelope. The local
SQLite spool uses synchronous transactions and defaults to **64 MiB of pending
payload bytes**; database metadata/journaling needs additional disk space. A full
spool stops collection explicitly without deleting queued batches.

An empty 202 or 200 response acknowledges delivery and removes that local batch.
Network/transient failures retain it for bounded retries or a later invocation.
Authentication, validation, redirects, unexpected response bodies, and identity
mismatches stop delivery for investigation. Tokens and response bodies are not logged.
Check the core's diagnostics for `processed`: acceptance confirms queueing only.
Core registration, ingestion, and persisted history have not been verified by
the scraper's standalone tests.

## Tests

```sh
# Offline fixtures, ingestion-contract compatibility, and mocked delivery behavior.
.venv/bin/pytest
.venv/bin/ruff check src tests
.venv/bin/ruff format --check src tests

# Real API requests: exact SKUs plus paginated rice/coffee discovery in Curitiba.
.venv/bin/pytest -m live --live
```

No PostgreSQL, Docker, or live shop is required by the default suite. Delivery
tests use HTTPX MockTransport and temporary local queue files.

The opt-in live tests make actual requests, validate the retailer's response
against `tests/fixtures/muffato-response-schema.json`, and run the parser. Removing
a required field or changing its type fails the tests. Additive fields are
allowed; changing prices, inventory, and promotion text are not snapshot failures.
An unavailable/blocked API or a missing selected SKU also fails, rather than
silently skipping. Run this command periodically to detect upstream changes;
no automated schedule or notification has been installed.

The saved fixture is a small projection of a real response; see
[fixtures/README.md](tests/fixtures/README.md). The ingestion schema is packaged
so installed collectors work outside a source checkout. Its equality with the
core-owned shared schema and both shared examples is tested. After a coordinated
contract change, refresh it from the repository root:

```sh
cp docs/contracts/ingestion/v1/schema.json scrapers/src/inflacao_scrapers/ingestion_v1.json
```
