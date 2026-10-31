# Web API v1

The React dashboard reads the Go core over same-origin HTTP. These two GET
endpoints are public and read-only; scraper ingestion and monitoring configuration
retain their existing bearer authentication. Never put the ingestion token in the
browser. Responses are JSON with `Cache-Control: no-store`. Each database request
has a ten-second deadline and follows request cancellation.

## Product catalog

`GET /api/v1/products?limit=100&after=123`

Returns canonical variants with in-stock, non-null price observations on at least
two distinct `America/Sao_Paulo` dates within the same listing, seller, store,
location and channel. Multiple payment offers and repeated collections on one
day count as one date; observations from different contexts do not combine to
meet this minimum. The rule applies both with and without `q` and
uses the observation's historical product assignment, not the listing's current
product. Never-available products, unknown-only availability, and in-stock
observations without prices do not qualify. This minimum applies across history;
a selected interval can still lack comparable endpoint prices, and the history
API continues to apply its price-condition and package-equivalence rules. A
product remains searchable after later becoming unavailable; the search promises historical prices, not current
stock. Direct history links still work for existing excluded products.
Variants remain distinct; monitoring groups are not products.

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
| `shopPrices` | Product-only context histories with `shopId`, `shopName`, `context`, `productUrl`, daily centavo `points`, and `included` in the matched panel. Unavailable/ineligible observations stay null. Overview returns `[]`. |
| `points` | Chronological `{date, value}` entries. Product values are integer centavos for the stated basis; overview values are integer basis points (803 = 8.03%). `null` means no selected price or incomplete coverage, never zero. |
| `latestPriceMinor` | Lowest eligible available context price on the most recent day with any price in the period, or `null`; always `null` for the overview. Independent of the matched panel. |
| `changeBasisPoints` | Core-calculated change, rounded to integer basis points. `null` for fewer than two usable days, a nonpositive product baseline, or an unrepresentable result. |
| `comparisonStart`, `comparisonEnd` | Requested start and end dates when a matched comparison exists and the dates differ; otherwise both `null`. No automatic baseline shifts. |
| `collectedAt` | Latest source `observed_at` among the selected product(s)' raw measurements **inside the requested period**, with explicit timezone; `null` if none. This is not ingestion receipt time or necessarily the date of the last selected price. |
| `stale` | Latest source observation is more than 48 hours old at request time. Old historical intervals can be stale by this definition. |
| `unknownConditions` | Some contexts use advertised prices with incompletely reported buying conditions. This does not mean the price is available unconditionally. The UI displays a notice. |
| `counts` | Observed `products`, distinct `shops`, `streams`, inclusive `days`, and `pricedDays` with usable plotted values. `includedProducts`, `includedShops` (distinct across products), and `includedStreams` count the matched panel; subtraction gives exclusions. Empty histories have zero counts. |
| `coverage` | Portuguese explanation of those counts. |
| `methodology`, `methodologyVersion` | Portuguese explanation and stable calculation identifier (`matched-equivalence-v3`). |

Money and percentages cross the JSON boundary as safe integers. Intermediate
averages and relative changes use exact rational arithmetic. Rounding occurs only
for displayed centavos/basis points, half away from zero. Extreme percentage
results outside JavaScript's safe integer range are `null`.

The UI displays available prices separately from matched variation. Product chart
prices use the matched panel's context averages within each shop, then equal shop
weights. The summary's lowest available price can differ from that chart and can
come from an excluded context. Percentage variation averages context relatives,
not ratios of those average price levels; they need not agree when initial prices
differ. Every eligible shop is counted once per product regardless of context count.

## Methodology: matched-equivalence-v3

1. Select historical product assignments and source observation times inside the
   requested inclusive São Paulo dates. Do not use current listing assignments.
2. Define each context by listing, seller, branch/store, postal code, city, state,
   and channel. Unknown context fields remain unknown.
3. Keep the existing price eligibility rules: equivalent canonical packages
   (including exact g/kg and ml/L conversion), or a single explicit kg/L/unit
   basis without a known package. Prefer explicitly unrestricted offers per
   context for the interval; otherwise accept advertised prices with unknown
   conditions, flagged as such. Explicit payment, club, coupon, installment and
   multiple-purchase restrictions remain excluded.
4. Use the latest observation per context/day, then its minimum eligible BRL
   offer, requiring in-stock availability. Ties use scraper identity and observation
   UUID. A later invalid or unavailable observation never falls back to an earlier
   price. Display these daily context prices even when excluded from variation.
5. Fix the comparison panel to contexts with valid prices on **both requested
   endpoints**, requiring a strictly positive starting price. Do not substitute
   another shop or move the baseline when an endpoint is missing. For a single
   date, prices are visible but no variation or comparison dates are returned.
6. Calculate each context's relative change from its own starting price. Average
   context changes within each shop, then average shops equally per product.
   The overview averages eligible products equally. A product with no matched
   contexts is excluded rather than invalidating other products.
7. Keep that panel fixed throughout the chart. Missing intermediate prices make
   the product point null; a missing included product makes the overview point
   null. Do not drop participants mid-series, interpolate, or carry prices forward.
   Endpoint coverage determines inclusion; gaps do not invalidate endpoint change.
8. Report total and included products, distinct shops and contexts, and days with
   usable plotted values. There is no minimum sample-size threshold; coverage
   must accompany the result. The overview is a monitored-sample index, not
   national inflation.

Explicitly unrestricted means `payment_method=any`, no installments,
`membership_required=false`, no membership program, `coupon_required=false`, and
`minimum_quantity=1`. If any such offer exists for a context in the interval,
that policy applies on every day. Otherwise the fallback accepts payment `any`
or `unknown`, no installments/program, membership/coupon false or unknown, and
minimum quantity one or unknown. Unknown terms never replace known unrestricted
terms. Free-text descriptions do not prove unrestricted access. Package prices
without known equivalent quantities and mixed explicit unit bases remain gaps.

Changing the interval can change the panel, basis and conditions policy. This
exploratory index tracks advertised prices in the collected sample; unknown
buying conditions can affect comparisons. It is not an expenditure-weighted
household basket, a purchasing-power estimate, or national inflation. No official
index comparison is supplied. The normalized observations retain source URLs,
measurement identities, observed packages and all conditions for investigation;
this API returns matched histories and eligible context price histories rather than raw offers.

## Errors

Errors use `{"error":"code"}` without internal database details.

| HTTP status | Code |
| --- | --- |
| 400 | `invalid_pagination`, `invalid_product_search`, or `invalid_history_request` |
| 404 | `product_not_found` |
| 500 | `data_unavailable` (details are logged internally) |

The browser preserves filters on failure, supports retry, aborts obsolete
requests, and does not fall back to demo values.

## Equivalence and owner administration

Search matches all member names and returns only the current representative of
each group. Member IDs remain accepted by history; global calculations count an
equivalence group once. Current membership is applied to the requested historical
period without rewriting source assignments. Reversing membership restores the
separate histories. Offer streams, conditions and coverage policies stay intact.
See the [admin contract](admin.md) for authentication, candidate evidence, generic
rules, review concurrency, audit and reversal endpoints.
