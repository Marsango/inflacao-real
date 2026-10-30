# Monitoring scope v1

`GET /api/v1/monitoring-targets` returns the core-owned collection scope for every
shop adapter. Authenticate using the same `Authorization: Bearer <INGESTION_TOKEN>`
as ingestion. Targets are global, rather than scraper- or shop-specific.

The core returns `200 application/json` with `Cache-Control: no-store`:

```json
{
  "schema_version": "1.0",
  "targets": [
    {
      "id": "cafe-moido",
      "name": "Café torrado e moído",
      "search_queries": ["cafe"],
      "required_terms": [["cafe"], ["moido", "po"]],
      "excluded_terms": ["soluvel", "capsula", "capsulas", "graos", "bebida", "filtro", "cafeteira", "leite"]
    }
  ]
}
```

Only enabled targets are returned, ordered by ID. An empty `targets` array means
there is no enabled collection work. Invalid/missing credentials receive
`401 {"error":"unauthorized"}`; database failures receive
`500 {"error":"monitoring_unavailable"}`. Database operations inherit request
cancellation and have a ten-second timeout.

## Discovery and matching

`id` is a stable, brand-independent target identifier. `name` is its display name.
`search_queries` are Portuguese discovery phrases: adapters translate these into
their shop's search API or website navigation and exhaust each query's pages.
Shop-specific category IDs, request parameters, pagination and parsing belong in
the adapter. Query results are candidates, not automatic product matches.

Apply `required_terms` and `excluded_terms` to each source SKU's complete name:

1. Lowercase, decompose Unicode using NFD, and remove combining marks (`Mn`).
2. Replace runs outside ASCII `a-z`/`0-9` with one space and trim outer spaces.
3. Normalize every rule phrase in the same way. Match complete phrases with word
   boundaries; `po` must not match `polido`, and `tipo 1` must not match `tipo 10`.
4. Every required group must have at least one matching alternative.
5. Any excluded phrase rejects the candidate.

Terms must contain letters or digits. Queries and required groups must be
nonempty. [Shared examples](matching-examples.json) exercise the same semantics
in both Go and Python. This is a deterministic name filter, not fuzzy matching or
a verified product taxonomy. Required details missing from a name prevent that
match. Unlisted exclusions or misleading source names can still misclassify a
listing; refine the central rules as evidence is collected.

There are no brand or package-size restrictions in v1: discover all matching
brands and individually sold package sizes. The adapter preserves original names,
brands, GTINs, sizes, availability, offers, store context, and source URLs through
the unchanged [ingestion v1 contract](../../ingestion/v1/README.md). Deduplicate
the same shop/SKU across queries and targets within a run. New runs rediscover
listings and take fresh measurements.

## Core ownership and identity

Migration 00006 seeds 16 targets covering the DIEESE basket's 13 food groups,
regional flour/sugar variants, and the previously monitored pasta. The
[basket scope and sources](basic-food-basket.md) document every target and adapter
coverage limits. Migration 00007 adds [21 everyday foods](everyday-foods.md),
and migration 00008 adds [20 foods and eight household essentials](foods-and-household.md).
Migration 00009 adds [12 family essentials](family-essentials.md), and migration
00010 adds [19 snacks, sweets and convenience foods](snacks-and-convenience.md),
bringing the fresh-database scope to **96 targets**. These are collection groups,
not an inflation basket or a declaration that their products are interchangeable.
Liquid and powdered milk, for example, remain different products.
Rice requires explicit `branco`/`polido` and `tipo 1`/`tipo1` evidence in its name.

The worker continues its existing GTIN/brand/variant/package identity matching.
It also applies enabled monitoring rules to each newly processed source name and
inserts matching `(product_id, target_id)` associations atomically with ingestion.
Associations group separate canonical products; they never merge brands, variants,
or package sizes. Ingestion of observations outside the current targets remains
valid. Older queued payloads still process without a contract change.

`product_monitoring_targets` is an additive catalog association, not a historical
snapshot of the target rules used by a scraper. Existing products acquire
associations when a new observation is processed; there is no initial backfill.
Rule edits and disabled targets do not erase existing associations or rewrite
price history. A future aggregate requires explicit methodology and rule history.

Manage targets through forward SQL migrations for now. No target-editing API or
manual shop-SKU registration is needed. With the default scraper configuration,
adding a target requires changes only in the core. Muffato and Festval search the
returned queries; Angeloni discovers all live leaf categories and filters the
catalog using the returned rules. Its optional `catalog_category_ids` override is
for restricted diagnostics, not a second product scope. Remove older category
allowlists to enable automatic discovery.
All adapters fetch the enabled scope once
at the beginning of each collection run; they must fail explicitly on an
unavailable or unsupported scope rather than use a hidden local fallback.

## Coverage and incomplete runs

Report matches per target, missing targets, parser errors and collection failures.
An exhausted search-page/request budget, repeated page, blocked source or failed
page makes the run incomplete; retain previously queued observations. No matching
listings means missing coverage, not an unchanged or zero price. Availability is
independent: matching out-of-stock listings are still observations.

Shop search is a moving catalog, not a transactional snapshot. Even an exhausted
query does not establish exhaustive retailer coverage: search indexing, source
limits and missing name attributes may hide listings. Surface these limits before
using the observations for comparisons or inflation metrics.
