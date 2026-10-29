# Muffato response fixtures

`muffato-products.json` contains two products from a live public VTEX response
retrieved on 2026-10-29 with `sc=8` (Curitiba – Champagnat):

```text
https://www.supermuffato.com.br/api/catalog_system/pub/products/search?ft=arroz&sc=8&_from=0&_to=19
```

The fixture retains the consumed product, SKU, seller, offer, promotion, and payment
fields. Unused descriptions, images, clusters, and platform metadata were removed.
Numeric source values are retained as JSON numbers and decoded as Decimal in tests.
Synthetic test mutations exercise missing prices, unavailable products, conditional
offers, and malformed responses without implying that those variations were observed live.

`muffato-response-schema.json` describes the **required response shape**, rather
than freezing prices or product names. Both saved-fixture tests and opt-in live
requests use it. Added fields are allowed; removed required fields or incompatible
types fail. Review actual API changes before updating this schema; do not regenerate
it blindly from an unexpected response.

## Weight prices and current SKU contents

`muffato-weight-products.json` and `muffato-package-products.json` are sanitized
projections of public Search API responses captured on 2026-10-29. Context:
`sc=8`, seller `1`, Curitiba–Champagnat online catalog, no selected delivery CEP.
The endpoint is `/api/catalog_system/pub/products/search` on
`https://www.supermuffato.com.br`, with the following bounded selections:

| Capture (UTC) | Query / range | Saved SKUs |
| --- | --- | --- |
| 19:04:07 | `ft=banana&_from=0&_to=4` | `186092` |
| 19:06:32 | `ft=patinho&_from=0&_to=4` | `269830`, fixed control `274553` |
| 19:11:51 | `ft=pao&_from=0&_to=49` | `145168` |
| 19:14:43 | `ft=ovos&_from=0&_to=9` | `215063`, `60834` |
| 19:14:47 | `ft=cafe&_from=0&_to=9` | `303169`, `304544` |

Product URLs and original selected SKU names remain in the fixtures. JSON numeric
literals were projected using Decimal, with no float conversion or money rounding.
Only Visa and Pix payment entries are retained; both original payment structures
and their totals remain available for comparison. No price tokens, session cookies,
customer details or credentials are retained. Test mutations are synthetic unless
explicitly identified as one of these captured records.

`muffato-weight-display.json` records independent, read-only headless Chromium DOM
excerpts for the three weighted product URLs, including the selected Champagnat
branch and the storefront's final-weight disclaimer. It also retains short
excerpts of deployed public shelf code with versioned source URLs. The wrapper
passes `commertialOffer.Price` and `ListPrice` directly to the price display;
the price component displays both without multiplying by `unitMultiplier`.
The first beef render timed out; its single bounded retry rendered successfully.

| Verified source meaning | Mapping |
| --- | --- |
| Banana `186092`: `kg`, multiplier `0.1500`, `isKit=false`, Price/ListPrice `13.4`; rendered R$13.40 | Base and reference `13.40/kg`; package null. `complementName="Peso aprox. 150g/un"` survives in conditions. |
| Beef `269830`: `kg`, multiplier `0.2500`, `isKit=false`, Price/ListPrice `58.7`; rendered R$58.70 | Base and reference `58.70/kg`; package null. |
| Bakery `145168`: `kg`, multiplier `0.2500`, `isKit=false`, Price/ListPrice `20.5`; rendered R$20.50 | Base and reference `20.50/kg`; package null. |
| Their `PaymentOptions.installmentOptions[].installments[].total`: `201`, `1467`, `512` minor units | Portion totals, distinct from the advertised kg amounts. Omit with `payment-basis` issues; never divide or relabel them. |
| Fixed meat `274553`: `un`, multiplier `1.0000`, name `320g`, Price `26.8` | `26.80/package`, contents `320 g`, count 1. Existing packaged payment semantics apply. |
| Eggs `215063`: `un`, multiplier `1.0000`, name ending `12un` | Twelve eggs in one sold carton: quantity `12`, unit `unit`, count 1. |
| `303169`: selected name contains `10g Choconilla`; `60834` repeats equivalent `500g` | Nonterminal and repeated equivalent SKU measurements are valid contents. |
| Capsule box `304544`: `52g 10 Unidades` | Component meaning is unverified; contents stay null. |

The `ean` fields on the loose samples (`34791`, `198196`, `14199`) are short
internal retailer codes, not valid-length GTINs. The verified Muffato weight
mapping maps digit-only codes shorter than eight digits to null GTIN, retaining
the SKU identity unchanged. Malformed full-length GTINs still fail validation.

The collected full responses exposed **no complete structured net-content
declaration** on these SKUs: only `name`, `nameComplete`, `complementName`, sale
units/multipliers and ordinary identifiers. No speculative `netWeight` field,
parent-product specification or shipping dimension is used as net content.
No additional detail request is added to collection. Structured-only extraction,
homogeneous multipacks and spelled-out unit words remain deferred until captured
source fields establish their meaning. Egg counts are enabled only in the verified
Muffato context. Angeloni and Festval have independently captured weight mappings
documented in [regional fixtures](stores/README.md); egg-count rules remain disabled
for those stores.

Fixed-package resolution receives the selected SKU and established sale basis.
It reconciles `items[].name` and `nameComplete`, supports `g`, `kg`, `ml`, `l`
(case insensitive, spaces and decimal comma), and compares equivalent mass/volume
declarations exactly. Conflicts and invalid contract quantities produce null
contents plus issues while retaining valid prices. Approximation, serving, drained
or gross weight, pricing references, bundles and unclear component counts cannot
supply exact contents. Parent/sibling sizes and prior observations are never used.
Ordinary missing contents remain null and are counted in `unknown_packages` run
diagnostics. Field-level source evidence is saved here for parser verification;
ingestion v1 does not persist those field paths or original responses.
