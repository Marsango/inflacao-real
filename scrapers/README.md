# Shop collectors

The registered adapters collect public VTEX catalogs for **Super Muffato,
Angeloni, and Festval** around Curitiba. The core supplies brand-independent
monitoring targets through its authenticated scope endpoint. Migration 00006
seeds 16 targets covering the DIEESE basket's 13 food groups, regional flour/sugar
variants, and pasta. See the [basket scope and sources](../docs/contracts/monitoring/v1/basic-food-basket.md).
Variable-weight meat, produce and bakery items still require adapter support;
the seed does not make their collection complete. Matching brands, variants and
package sizes stay separate products.
These groups are not a statistically representative inflation basket.

| Adapter | Verified context | Discovery |
| --- | --- | --- |
| `super-muffato` | Champagnat online branch, sales channel `8`; no CEP selected | Paginated target searches |
| `angeloni` | Delivery catalog for CEP `80730201`, mapped to Angeloni Bigorrilho | Food-category browsing, filtered by core targets |
| `festval` | Delivery catalog for CEP `80730201`, mapped to regional seller `meufestvaldark` | Paginated target searches on sales channel `2` |

The two delivery adapters resolve their configured CEP through the public regions
endpoint at the start of every run. The response must contain the configured
`regional_seller_id`; a missing/ambiguous mapping stops that scraper before price
collection. The resolved region is sent explicitly in the request and public
VTEX segment cookie. Observations preserve CEP, city/state, `channel=delivery`,
and `store_id=region:<resolved region>/seller:<regional seller>`. The price
response's seller ID remains `1`; it is preserved separately in `seller_id`.
This records a regional online catalog; checkout prices and physical shelf prices
require separate verification. Muffato's current context remains `online`, with
a null CEP.

## Setup

Run from `scrapers/`, with Python 3.12 or newer:

```sh
python3 -m venv .venv
.venv/bin/pip install --no-cache-dir -e '.[dev]'
cp scrapers.example.toml config.toml
```

This makes the Python package and `inflacao-scraper` command available inside
the local virtual environment. Editable mode (`-e`) uses source changes directly;
`[dev]` adds pytest and Ruff. No service, scheduler, browser, or system-wide
installation is needed. Database access is not part of scraper operation or tests.

`config.toml` and `data/` are ignored by Git. Set a unique `scraper_id` for each
independent installation. Keep the ingestion token only in the environment.

## Shared runner and adapter registration

The same command runs every enabled entry in `config.toml`:

```sh
.venv/bin/inflacao-scraper --config config.toml collect
```

Use one `[[scrapers]]` table per shop/store configuration. Each table has its own
adapter, scraper/shop identity, core URL, request settings, branch selection,
and fallback spool. The adapter name selects an implementation from
[`ADAPTERS`](src/inflacao_scrapers/adapters/registry.py). Muffato, Angeloni and
Festval are registered. The complete three-shop setup is in
[`scrapers.example.toml`](scrapers.example.toml). Individual configurations are in
[`angeloni.example.toml`](angeloni.example.toml),
[`festval.example.toml`](festval.example.toml), and
[`muffato.example.toml`](muffato.example.toml). For example:

```toml
[[scrapers]]
adapter = "super-muffato"
enabled = true
scraper_id = "local-collector"
core_url = "http://127.0.0.1:8080"
ingestion_token_env = "INGESTION_TOKEN"
sales_channel = "8"
store_id = "sales-channel:8"
city = "Curitiba"
state = "PR"

[[scrapers]]
adapter = "another-shop"
enabled = false
# After registering this adapter, add its required settings and enable it.
```

Runs are sequential. Each enabled scraper gets a new HTTP client, run ID, and
listing-deduplication state. A collection or delivery failure stops that scraper;
the runner continues with the next entry. The final summary reports the number of
scrapers and failures, and the command exits nonzero if any scraper failed.
Unknown enabled adapters, invalid settings, and conflicting spool paths fail
configuration validation before any scraper starts. Disabled entries require
only `adapter` and `enabled = false`; their adapters need not be installed.
All-disabled configurations are a successful no-op.

`ingestion_token_env` names the environment variable containing that scraper's
core token; it defaults to `INGESTION_TOKEN`. Tokens stay out of TOML files and
shop requests. The current core authorizes one `INGESTION_SCRAPER_ID` and a set of
`INGESTION_SHOP_IDS`; entries sending to the same core must use that authorized
scraper identity and token, with each shop registered and allowed. Separate core
instances can have different identities and token environment variables.

Runner entries default to `data/<shop_id>/<scraper_id>/spool.sqlite3`, relative to
the configuration file. Set distinct `spool_path` values explicitly when several
entries use the same shop and scraper identity. `deliver`, `status`, and diagnostic
`discover` also run across all enabled entries using their own settings.

Existing flat Muffato TOML files still work, default to `adapter = "super-muffato"`,
and retain `data/spool.sqlite3`. When migrating an existing queue to `[[scrapers]]`,
retain its explicit `spool_path`; the new example does this for Muffato.

To add a shop without changing the runner:

1. Implement pure parsing and bounded fetching in its own package. Subclass the
   shared `Settings` model for that shop's required options.
2. Define an [`Adapter`](src/inflacao_scrapers/adapters/__init__.py) with its name,
   settings model, `search_pages(requester, settings, target, query)`, and
   `discover(requester, settings, query, limit)` functions. Searches yield shared
   `ParsedPage` instances containing validated observations and parsing issues.
   An optional `prepare` callback resolves per-run context. An adapter that browses
   a whole catalog supplies `catalog_pages(requester, settings, scope)` instead
   of `search_pages`; the runner counts matches for every core target.
   Use fresh observation IDs/timestamps and report incomplete pagination as errors.
3. Import its adapter into `adapters/registry.py` and add it to `ADAPTERS` under
   its name. Registration is explicit; adding an unregistered Python file does
   not make it runnable.
4. Add an enabled `[[scrapers]]` table with that adapter's settings and authorize
   its identity/shop in the core. The existing `collect` command discovers the
   registration, collects its pages, and uses shared batching and delivery.

See [`muffato/adapter.py`](src/inflacao_scrapers/muffato/adapter.py) for the first
implementation. Adapter names are included in submitted collector provenance.

## Collection

```sh
.venv/bin/inflacao-scraper --config config.toml discover --query arroz --limit 50
# Start the core and export its matching INGESTION_TOKEN before collection.
.venv/bin/inflacao-scraper --config config.toml collect
.venv/bin/inflacao-scraper --config config.toml status
```

Discovery prints SKU IDs, names, package sizes, availability, and prices without
queueing measurements. It is an ad hoc diagnostic command, independent of the core.

For each enabled scraper, `collect` authenticates to `GET /api/v1/monitoring-targets`
once per run. Muffato and Festval search each target's phrases. They follow pagination, filter each
SKU using the central required/excluded name terms, and collects every matching
brand and individually sold package size. A SKU found by several queries or
targets is measured once per run. Brand and variant comparisons remain the core's
responsibility. See the [monitoring v1 contract](../docs/contracts/monitoring/v1/README.md).

Each page's validated observations are batched and sent directly to the core
before requesting another page. Successful delivery creates no local storage.
If delivery fails after bounded retries, the exact attempted batch is saved in
the scraper's configured spool and that scraper stops with a nonzero result. Use `deliver`
to retry saved batches; `collect` does not drain an existing backlog.
Every new measurement receives a new observation ID and UTC timestamp, even if
the price is unchanged. Run and submission IDs are printed for diagnosis. An
incomplete run exits nonzero; batches already accepted by the core remain there.
Restarting collection creates a new run. An interruption before a failed batch
is saved can lose that in-memory batch.

The core seeds the initial scope through migration 00006; add/refine targets with
forward migrations. Remove the old `sku_ids` field from existing local TOML files.
`collect` now needs `INGESTION_TOKEN` and a reachable core. It never falls back to
a local product list if scope retrieval or validation fails. Credentials are sent
only to the configured core, never to shops; redirects are not followed.

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

### Angeloni and Festval regionalization

The defaults use the public business-address CEP `80730201` in Bigorrilho. On
2026-10-29 the stores' public `GET /api/checkout/pub/regions` responses mapped
this CEP to `superangeloni24` (Angeloni Bigorrilho) and `meufestvaldark` (Festval).
These are delivery-region sellers, not pickup-point IDs. The regional mappings
are checked again on each invocation, rather than hardcoding a region token.
Changing the CEP requires verifying and configuring the matching regional seller.

Festval's homepage segment selected sales channel `2`. The adapter also preserves
the storefront's `utm_source=curitiba` segment field. Default unregionalized
search responses reported offers unavailable; applying the resolved regional
segment returned in-stock prices in live checks. The adapter never falls back
to an unregionalized price when resolving the configured context fails.

Angeloni's [published robots rules](https://super.angeloni.com.br/robots.txt)
exclude `ft`/`fq` keyword-filter URLs. Its adapter uses the public
`/_v/api/intelligent-search/product_search/<category facets>` catalog endpoint
with `count`, `page`, `sc`, and `regionId`. It uses no `ft`, `fq`, `query`, or `map`
query parameters. Each run loads the public category tree and resolves configured
food leaf-category IDs into browsing paths. Each selected category is traversed
once for all enabled core targets, then source names are filtered locally before
parsing offers. Source pages are not replayed from a previous run.

The defaults cover rice, beans, coffee, flour/corn flakes, milk/powdered milk,
butter/margarine, oils, sugar, fruit, vegetables, beef, bread and pasta. These are
retailer category partitions, not a replacement for the core's name rules. Add
appropriate `catalog_category_ids` when expanding scope beyond those categories.
Missing configured categories or parent categories stop collection explicitly.
Product overlap across categories is deduplicated within the run.

VTEX rejects pages after page 50, so the adapter enforces that bound per category.
`max_catalog_pages` defaults to 1000 as an overall page budget across selected
categories. Reaching either bound, repeated pages, missing totals and prematurely
short pages are explicit incomplete runs. Already accepted batches remain in the core.
`discover` also browses Angeloni's configured categories, stopping after its requested number
of matching observations; use an individual config to inspect only one shop:

```sh
.venv/bin/inflacao-scraper --config angeloni.example.toml discover --query leite --limit 5
.venv/bin/inflacao-scraper --config festval.example.toml discover --query arroz --limit 5
```

Shared fetching, parsing, pagination and regionalization now live in `vtex/`.
Saved fixtures cover both response formats; Angeloni's Intelligent Search format
uses `AvailableQuantity` when `IsAvailable` is absent. Both retain unavailable
displayed offers, promotions and payment alternatives. Equivalent repeated package
sizes agree; conflicting declarations stay null with a collection issue.
Angeloni and Festval also support advertised kilogram prices in their verified
Curitiba API contexts. Independent storefront checks establish the price unit;
the comparison may use another Curitiba pickup location. API availability and
location fields always come from the configured regional response, without
copying a storefront's pickup selection or assuming identical branch inventory.

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
money strings. All three adapters retain individually sold packaged goods
(`measurementUnit=un`, `unitMultiplier=1`). The SKU-aware contents resolver uses
unambiguous measurements in the selected SKU's `name` and `nameComplete`, including
nonterminal sizes and equivalent repeated measurements. Units are `g`, `kg`, `ml`
and `l`, with spaces, case differences and decimal commas supported. Conflicting
or invalid quantities produce null contents and an issue while valid prices survive.
Unknown sizes stay null on each fresh observation and appear in the run's
`unknown_packages` count. Parent/sibling metadata and prior sizes never fill gaps.

Muffato's verified sales channel `8`, seller `1`, without a delivery CEP also
supports advertised kilogram prices with positive multipliers up to one. Captured
produce, beef and bakery listings were checked against the rendered Champagnat
storefront. `Price` and `ListPrice` are kilogram amounts; the multiplier is never
applied to them. Package is null even when a name contains `1kg` or an approximate
portion weight. The latter wording is preserved in offer conditions. Short numeric
retailer codes on these loose products become null GTINs; malformed full-length
GTINs still fail validation. Kits, other units, invalid multipliers and unverified
shop/context mappings remain explicit issues.

Angeloni enables this mapping on channel `1` with regional seller
`superangeloni24`; Festval on channel `2` with `meufestvaldark` and the
`curitiba` marketing segment. Both require a freshly resolved region, seller `1`,
and configured Curitiba/PR context. Captured fractional multipliers and rendered
`/ KG` or `/kg` labels establish the advertised basis independently of portion
totals. Angeloni's sampled offers have no payment alternatives. Festval's portion
totals remain omitted under the same explicit diagnostic rule as Muffato's.
Festval's Carlos de Carvalho pickup is in Bigorrilho, Curitiba; exact pickup
equality is not required by the city-wide scope. Angeloni's storefront branch
was not independently identified; collection location remains established by
the verified Bigorrilho API region. Storefront availability does not overwrite it.
See [regional source evidence](tests/fixtures/stores/README.md).

Weighted `PaymentOptions` totals describe portions and have no verified per-kg
mapping. They are omitted with a `payment-basis` diagnostic: independently valid
base prices are delivered, but the run reports incomplete collection. No conversion
from estimated totals, guessed promotional minimum or final checkout total is made.
Missing prices and explained unavailable placeholders still produce empty offers.
The storefront warns that final weighed totals can vary.

Muffato's verified egg-carton names ending in `12un` (or another explicit count)
map to quantity in `unit`, count 1. Homogeneous multipacks, mixed kits, bonus
bundles and structured-only sizes remain deferred; sampled responses contained no
verified complete SKU-specific structured net-content fields. No extra per-product
request was added. See the [source mapping and provenance](tests/fixtures/README.md)
for supported rules and remaining unknown-size coverage. Reliable size history is
an input to later shrinkflation analysis; no detector or replacement matching is added.

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
`INGESTION_SHOP_IDS=super-muffato,angeloni,festval`. An administrator must register the shops before
the worker processes observations; the scraper itself never accesses PostgreSQL:

```sql
INSERT INTO shops (id, name, website_url)
VALUES ('super-muffato', 'Super Muffato', 'https://www.supermuffato.com.br'),
       ('angeloni', 'Angeloni', 'https://super.angeloni.com.br'),
       ('festval', 'Festval', 'https://www.festval.com')
ON CONFLICT (id) DO NOTHING;
```

Use the existing core setup instructions in [core/README.md](../core/README.md).
With the matching token exported under the configured `ingestion_token_env`, `collect` sends newly
collected prices automatically. To retry batches saved after delivery failures, run:

```sh
.venv/bin/inflacao-scraper --config config.toml deliver
```

Delivery reuses the exact attempted UTF-8 body during retries and, for failed
batches, after restarts. Batches are limited to 100 observations and 1 MiB,
including their envelope. Each local
SQLite spool uses synchronous transactions and defaults to **64 MiB of pending
payload bytes**; database metadata/journaling needs additional disk space. A full
spool stops collection explicitly without deleting queued batches.

An empty 202 or 200 response acknowledges delivery. New successful batches are
never stored locally; retrying a saved batch removes it only after acknowledgement.
Network/transient failures use bounded retries. Exhausted retries, authentication,
validation, redirects, and unexpected response bodies save a new batch locally
and stop collection for investigation. An unsuccessful `deliver` leaves existing
batches queued. Identity mismatches also stop delivery. Tokens and response bodies
are not logged.
Check the core's diagnostics for `processed`: acceptance confirms queueing only.
Core integration tests separately exercise the shared kilogram example through
authenticated acceptance, worker processing, history persistence and retry
deduplication using disposable PostgreSQL Testcontainers. They also verify fixed
size/component reductions and later unknown sizes. This does not verify delivery
to a running deployment or the historical queued live measurements below.

### Pending end-to-end verification

Two fixed-scope Muffato runs collected 48 observations on 2026-10-29 with fresh
IDs and timestamps. Those observations were queued locally; their delivery and
persisted history have not been verified. These runs predate core-owned target
discovery and automatic delivery.

After registering the shops and configuring authorization, deliver any pending
batches and verify two collection runs against the same scope:

- Accepted submissions reach `processed`; investigate retries and failures through
  core diagnostics.
- Persisted prices, packages, conditions and store context match the observations.
- Fresh measurements retain distinct IDs and timestamps while compatible products
  and listings reuse their catalog records.
- Retrying an interrupted submission preserves its payload and identities without
  duplicating history.

Inspect PostgreSQL manually for this operational check. Automated persistence
tests belong to the core and must use disposable Testcontainers with automatic
cleanup. Scrapers continue to use HTTP only.

Further work includes additional verified shop contexts, structured net contents
and multipacks, a core history API and web view,
scheduling, automated raw-payload retention, and collection-coverage reporting.
Aggregate metrics require documented baskets, weighting, comparison periods and
missing-data treatment before inflation or official-index comparisons.

## Tests

```sh
# Offline fixtures, ingestion-contract compatibility, and mocked delivery behavior.
.venv/bin/pytest
.venv/bin/ruff check src tests
.venv/bin/ruff format --check src tests

# Real API requests across all three stores.
.venv/bin/pytest -m live --live
# Only the new adapters: regional prices and Angeloni pagination.
.venv/bin/pytest tests/stores/test_live.py -m live --live
# Bounded fresh produce, beef and bakery checks for the verified Muffato mapping.
.venv/bin/pytest tests/weight/test_live.py --live
# Six regional produce/beef/bakery checks, preserving each shop's collection route.
.venv/bin/pytest tests/stores/test_live.py -k advertised_kilogram --live
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
[fixtures/README.md](tests/fixtures/README.md) and
[regional-store fixtures](tests/fixtures/stores/README.md). The ingestion schema is packaged
so installed collectors work outside a source checkout. Its equality with the
core-owned shared schema and all three shared examples is tested. After a coordinated
contract change, refresh it from the repository root:

```sh
cp docs/contracts/ingestion/v1/schema.json scrapers/src/inflacao_scrapers/ingestion_v1.json
```
