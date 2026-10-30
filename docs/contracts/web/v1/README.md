# Web API v1

The React dashboard reads the Go core over same-origin HTTP. These two GET
endpoints are public and read-only; scraper ingestion and monitoring configuration
retain their existing bearer authentication. Never put the ingestion token in the
browser. Responses are JSON with `Cache-Control: no-store`. Each database request
has a ten-second deadline and follows request cancellation.

## Product catalog

`GET /api/v1/products?limit=100&after=123`

Returns canonical variants with historical observations, including historical
assignments that differ from a listing's current product. Unobserved catalog
entries are excluded. Variants remain distinct; monitoring groups are not products.

```json
{
  "items": [{"id": "124", "label": "Arroz branco 5 kg · Marca Exemplo"}],
  "nextCursor": null
}
```

IDs and cursors are decimal **strings**, preserving PostgreSQL bigint precision.
Results sort by numeric ID ascending. `limit` defaults to 100 and accepts 1–200.
`after` is an exclusive positive canonical ID; omit it for the first page.
`nextCursor` is the last returned ID if another page exists, otherwise `null`.
An empty catalog has `items: []`.

Add `q` to search names and brands, for example
`GET /api/v1/products?q=cafe+uniao&limit=20`. Search ignores case and Portuguese
accents, matches every word in any order, and supports word prefixes (`torr`
finds `torrado`). Punctuation is treated as separators, never as search operators.
Quantities such as `500g` and `500 g` are tokenized alike. This is prefix search,
not typo correction or substring matching. An explicitly supplied `q` must contain
at least two letters/digits and at most 100 Unicode characters after trimming.
Search defaults to 20 results; the same limit and ID cursor bounds apply.
Names and brands have a generated PostgreSQL search document and a GIN index.
Variants and historical-assignment rules remain unchanged.

The React client makes **no catalog request on page load**. It waits for at least
two letters/digits and 300 ms without typing, then requests a single page of at
most 20 matches. It never follows cursors automatically or accumulates the full
catalog. When more matches exist, users refine their search. Obsolete searches
are aborted and their responses ignored; selecting a result updates the shareable
product filter. Shared product URLs obtain the selected label from history.

Pagination is not a transaction snapshot; newly ingested products may appear on
later pages or after reloading the catalog.

## History

`GET /api/v1/history?product=124&start=2026-10-27&end=2026-10-29`

Use `product=global` for the overview. All three parameters are required. Dates
are real `YYYY-MM-DD` calendar dates, inclusive in `America/Sao_Paulo`. Start must
not exceed end; the difference is limited to 366 days (up to 367 daily points),
covering the UI's twelve-calendar-month preset across leap years. An existing
product with no observations in the period returns an empty history; an unknown
product returns 404. An empty database returns an empty overview.

See the [example response](history.example.json), also used by frontend tests.

| Field | Meaning |
| --- | --- |
| `period`, `timeZone` | Requested inclusive dates and the daily observation timezone. |
| `title`, `unit` | Display title; `BRL` for product prices or `percent` for the overview. |
| `priceBasis` | `package`, `kg`, `l`, or `unit` for the product's selected basis; `null` for the overview or an undetermined basis. |
| `points` | Chronological `{date, value}` entries. Product values are integer centavos for the stated basis; overview values are integer basis points (803 = 8.03%). `null` means no selected price or incomplete coverage, never zero. |
| `latestPriceMinor` | Last non-null daily product price in the requested period, or `null`; always `null` for the overview. |
| `changeBasisPoints` | Core-calculated change, rounded to integer basis points. `null` for fewer than two usable days, a nonpositive product baseline, or an unrepresentable result. |
| `comparisonStart`, `comparisonEnd` | Actual first and last usable days underlying the summary. Both `null` without usable days; identical for one usable day, which still has no variation. These may differ from requested endpoints. |
| `collectedAt` | Latest source `observed_at` among the selected product(s)' raw measurements **inside the requested period**, with explicit timezone; `null` if none. This is not ingestion receipt time or necessarily the date of the last selected price. |
| `stale` | Latest source observation is more than 48 hours old at request time. Old historical intervals can be stale by this definition. |
| `unknownConditions` | Some contexts use advertised prices with incompletely reported buying conditions. This does not mean the price is available unconditionally. The UI displays a notice. |
| `counts` | Products, distinct shops, fixed contexts, inclusive days and days with usable plotted values. Empty histories have zero counts. |
| `coverage` | Portuguese explanation of those counts. |
| `methodology`, `methodologyVersion` | Portuguese explanation and stable calculation identifier (`fixed-panel-v1`). |

Money and percentages cross the JSON boundary as safe integers. Intermediate
averages and relative changes use exact rational arithmetic. Rounding occurs only
for displayed centavos/basis points, half away from zero. Extreme percentage
results outside JavaScript's safe integer range are `null`.

The UI explains missing variation using `counts.pricedDays`: zero comparable days
means no complete coverage; one comparable day needs another day for a change.
The overview can remain unavailable even after several collections if no day
covers its entire fixed panel. The UI offers product search in that case and never
replaces unavailable variation with zero or changes the core's calculation.

## Methodology: fixed-panel-v1

1. Select observations by historical `price_observations.product_id` and source
   observation time. Do not follow today's `shop_products.product_id`. Rows with
   unknown historical canonical identity do not enter these series.
2. Fix the set of contexts observed anywhere in the requested interval: listing,
   seller, branch/store ID, postal code, city, state, and channel. Missing context
   fields remain unknown; they are not claims about a physical location. Every
   observed context remains in the panel, including unavailable contexts.
3. Fix a price basis per product. If the canonical package is known, use package
   prices only, requiring an equivalent observed quantity, dimension and component
   count. Grams/kilograms and milliliters/liters compare exactly. With no canonical
   package, allow a single explicitly observed basis (`kg`, `l`, or `unit`). Mixed
   bases or package prices of unknown quantity cannot establish equivalence and
   produce gaps. There is no implicit conversion of portion totals to per-kg prices.
4. Fix a buying-conditions policy per context for the entire interval. Prefer
   explicitly unrestricted offers if any exist: `payment_method=any`, no
   installments, `membership_required=false`, no membership program,
   `coupon_required=false`, and `minimum_quantity=1`. Other days must meet that
   same policy; unknown terms never replace known unrestricted terms.
5. When a context has no explicitly unrestricted offers, allow the announced
   base price with no **known** restrictions: payment `any` or `unknown`, no
   installments/program, membership/coupon false or unknown, minimum quantity one
   or unknown. Flag `unknownConditions`. Known restricted payment methods, member,
   coupon and multiple-purchase offers are excluded. Free-text descriptions and
   unknown fields are not proof of unrestricted access. The current VTEX scrapers
   deliberately report these fields as unknown, so their base prices remain
   inspectable with this limitation instead of being silently called unconditional.
6. For each context/day, use the latest observation, then the minimum eligible
   BRL offer in that observation, requiring `in_stock`. Equal-time observations
   use scraper identity then observation UUID as a deterministic tie-breaker. A
   later unavailable/no-offer/invalid observation never falls back to an earlier
   price. Reference prices and conditional offers do not define the series.
7. A daily product price is the equal-weight arithmetic mean across its fixed
   contexts, only when every context has a selected price that day. Missing days
   and incomplete coverage remain `null`; no carry-forward or interpolation.
   Product variation compares the first and last usable unrounded means.
8. The overview fixes all canonical products observed in the interval. Its
   baseline is the first day with a positive usable price for **every** product.
   Each subsequent complete day is the equal-weight mean of product price
   relatives to that baseline, minus one, expressed in basis points. Missing
   products never change weights or count as unchanged prices. There may be no
   overview at all if no day covers the entire panel. Selecting a product can
   still reveal its history.

Changing the interval can change the panel, basis and conditions policy. This
exploratory index tracks advertised prices in the collected sample; unknown
buying conditions can affect comparisons. It is not an expenditure-weighted
household basket, a purchasing-power estimate, or national inflation. No official
index comparison is supplied. The normalized observations retain source URLs,
measurement identities, observed packages and all conditions for investigation;
this API returns consolidated histories rather than raw offers.

## Errors

Errors use `{"error":"code"}` without internal database details.

| HTTP status | Code |
| --- | --- |
| 400 | `invalid_pagination`, `invalid_product_search`, or `invalid_history_request` |
| 404 | `product_not_found` |
| 500 | `data_unavailable` (details are logged internally) |

The browser preserves filters on failure, supports retry, aborts obsolete
requests, and does not fall back to demo values.
