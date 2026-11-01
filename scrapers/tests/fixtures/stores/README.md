# Regional-store response fixtures

## Nagumo Vila Curuçá pickup — 2026-11-01

The `nagumo-*` fixtures come from the public Salesforce Commerce Cloud storefront
after explicitly selecting pickup at store `13`, Praça Francisco Pereira 80,
Vila Curuçá, São Paulo/SP, branch CEP `08030040`. No customer login is used.

- `nagumo-context.json` and `nagumo-selection.json` preserve the public branch list
  with store 13 active and the successful pickup-selection response. No cookies,
  session headers, customer information or tracking tokens are retained.
- `nagumo-rice.json`, `nagumo-rice-next.json`, `nagumo-rice-last.json` and
  `nagumo-empty.json` project first/second/final/empty `Search-UpdateGrid` responses,
  preserving counts, page metadata, next links and the product fields consumed.
  The rice result reported 577 products; its final listing, SKU `252724`, was
  unavailable. These are captured source cases, not synthetic stock fixtures.
- `nagumo-samples.json` projects nine products: rice, beans, milk, coffee,
  detergent and four variable-weight bananas. JSON numeric lexemes remain exact;
  fixture preparation never converts money through binary floating-point values.
- `nagumo-verification.json` records each selected source URL/capture time and
  independent rendered excerpts in the same pickup context, including visible
  membership labels, ordinary prices and advertised kg/portion amounts.
- `nagumo-submission.json` contains ingestion-v1 output built by the adapter from
  those nine source records, with their original capture times. Go tests replay
  it on two **synthetic** dates in disposable PostgreSQL Testcontainers, checking
  worker persistence, search/history, pickup context, names, links and membership
  conditions. This is not evidence of two live observation dates.

The adapter keeps regular and `NGM_13_M` Meu Nagumo prices as separate offers.
Activation, payment, coupon and minimum-quantity conditions stay unknown.
Variable-weight prices use advertised kg amounts; rounded portion totals and
average weights never determine the price basis or fixed package contents.
No source GTIN is verified in these grids. Unsupported conditions/units,
malformed/unknown prices and lost context are explicit synthetic mutations in
tests. Runner tests substitute small pagination totals to exercise delivery.

See [Nagumo source mapping, setup and limits](../../../../docs/plans/nagumo-retailer.md).

## Mambo, São Paulo — 2026-11-01

The `mambo-*` fixtures add a separate São Paulo city context. They do not change
the Curitiba fixture provenance below.

- `mambo-region.json`: public
  `https://www.mambo.com.br/api/checkout/pub/regions?country=BRA&postalCode=01310200&sc=1`;
  region `v2.B032579D3A36A563BE9985A52457C9DD`, regional seller `mambodelivery215`.
  The CEP comes from the [public MASP address](https://www.masp.org.br/visite).
  It establishes service to a capital-city CEP, not a physical branch identity.
- `mambo-products.json`: seven products projected from the public
  `/api/catalog_system/pub/products/search` route with channel `1`, resolved
  `regionId` and public `vtex_segment`. Searches: `arroz branco`, `leite integral`,
  `feijao`, `cafe`, `detergente`. This combined fixture is not one actual page.
  Examples: rice SKU `288357` (5 kg, BRL 34.98), milk `283934` (1 L, 9.29), beans
  `6112` (500 g, 11.98), coffee `2775` (500 g, 26.99, reference 49.98), detergent
  `5415` (500 ml, 2.98, reference 3.59). Unavailable SKUs `755` and `5418` have zero
  placeholders; those become empty offer arrays. All expose offer seller `1`.
- `mambo-weight-candidates.json`: five verified kg candidates, refreshed from
  the same API/context at 01:22 UTC: banana `1260` (`kg`, multiplier 0.18, Price
  9.99), beef `7425` (`kg`, 0.6, 79.98), bread `11549` (`kg`, 0.08, 19.9), melon
  `287726` (`kg`, 1.5, 12.98) and melon `9274` (`kg`, 3, 14.98). Payment totals
  are respectively 179, 4798, 159, 1947 and 4494 minor units for approximate
  portions. They are not per-kg payment prices.
- `mambo-weight-verification.json`: source capture URLs/times and independent
  rendered excerpts from 01:22–01:25 UTC, after selecting Entrega Agendada and
  entering CEP `01310200` in a fresh Botasaurus browser session. The public session
  API confirmed country, CEP, region and sales channel, matching the scraper.
  Each product displayed an explicit `/Kg` amount matching the catalog `Price`,
  alongside the estimated portion amount. Thus kg prices remain unscaled and
  package contents remain null. No login, purchase or cart mutation was performed.
  The 3 kg upper portion bound is based on the largest checked listing; unsupported
  contexts, larger portions, kits and other units remain explicit failures.
- `mambo-weight-submission.json`: five validated ingestion-v1 observations built
  from those captures, preserving each source URL/time. Not delivered to production.
  Go tests replay them with synthetic dates and IDs, verifying per-kg history and
  that estimated payment totals/fixed package contents are not introduced.
- `mambo-submission.json`: one validated Python ingestion-v1 submission captured
  at **2026-11-01T01:02:00.017765Z**, through the actual collector HTTP client and
  regional preparation. It contains rice SKU `288357`, its exact fetched source
  URL, collector name, observation ID and actual region. It was not submitted to
  a running core. Go tests replay it with synthetic dates/identities to verify
  ingestion, retained provenance and two-date catalog eligibility.

The source fixtures retain only relevant product/SKU/offer fields. Decimal numeric
literals were decoded with `Decimal` and serialized without binary floats. Payment
options retain only public names/groups and installment totals/counts; images,
price tokens and unrelated fields are removed. Source URLs in this note omit
per-request cache-busting UUIDs; the captured submission keeps its actual URL.
No account cookies or ingestion tokens are saved.

The public `robots.txt` excludes account, checkout and storefront search pages;
the adapter uses the public API route, with bounded requests and no storefront
`/busca/` traversal. [Terms](https://www.mambo.com.br/termos-de-uso) were reviewed.
See [activation and remaining blockers](../../../../docs/plans/sao-paulo-retailers.md).

## Curitiba — 2026-10-29

Captured on 2026-10-29 from public, unauthenticated VTEX endpoints. Both stores
were regionalized for the public Bigorrilho business-address CEP `80730201`.

- `angeloni-region.json`: `GET https://super.angeloni.com.br/api/checkout/pub/regions?country=BRA&postalCode=80730201&sc=1`;
  seller `superangeloni24`, named Angeloni Bigorrilho.
- `festval-region.json`: the same public regions endpoint on `www.festval.com`,
  with `sc=2`; seller `meufestvaldark`.
- `angeloni-products.json`: two products projected from
  `https://super.angeloni.com.br/_v/api/intelligent-search/product_search`,
  requested with `count`, `page`, `sc`, `regionId` and the public regional segment
  cookie. The fixture's total is reduced to its two saved products. Offers expose
  `AvailableQuantity`, without `IsAvailable`; product links are relative.
- `angeloni-categories.json`: projection of the public `/api/catalog_system/pub/category/tree/5`
  response, refreshed on 2026-10-29 for the everyday-food expansion. Keeps paths
  for 38 food/household leaf categories and one chocolate category used in
  pipeline tests. This is a bounded fixture projection; the default scraper
  discovers every leaf in the full live tree instead of using a category allowlist.
  Names, IDs, parentage and
  `hasChildren` flags retain their source values.
  The 11 added IDs are `83` (Aves), `144` (Iogurte), `149` (Queijos),
  `182` (Ervas e Temperos), `187` (Ovos), `190` (Verduras),
  `267` (Biscoitos Salgados), `278` (Aveia), `298` (Sardinha),
  `322` (Molho de Tomate), and `324` (Sal). The category response establishes
  navigation paths; it does not verify prices or availability of the new foods.
  The food/household expansion adds `88` (Carnes Suínas), `86` (Peixes e Frutos Do Mar),
  `150` (Requeijão), `245` (Vinagre e Aceto), `318` (Extrato e Polpa de Tomate),
  `163` (Papel Higiênico), `166` (Sabonetes em Barra), `168` (Creme Dental),
  `173` (Absorvente), `205` (Detergente e Lava-louças), `207` (Lava-Roupas),
  and `196` (Água Sanitária), checked through the same public category endpoint.
- `festval-products.json`: three rice products projected from
  `https://www.festval.com/api/catalog_system/pub/products/search`, queried for
  `arroz`, with `sc=2`, regional context and its `utm_source=curitiba` segment.
  Includes in-stock packaged rice, unavailable displayed kit offers, promotions
  and payment terms. Repeated/contradictory sizes in source names stay unknown.

Only fields used by extraction are retained. Prices and quantities remain exact
JSON numbers; no binary floating-point conversion was used to create fixtures.
Images, response price tokens and unrelated merchandising fields are omitted.
Region records contain public store mappings, with no customer session or core
ingestion credentials. Fixture replay tests use explicit timestamps and make no
network calls. Opt-in live checks resolve the current region again and fetch fresh
responses; they neither deliver observations nor write to the local spool.

## Weight mappings: Curitiba collection scope

`angeloni-weight-candidates.json` and `festval-weight-candidates.json` retain
additional sanitized public responses captured at 19:32 UTC on 2026-10-29,
including fixed-package controls. Their `*-weight-verification.json` companions
record exact capture times, expected public regional context and read-only
rendered-page excerpts. Decimal numeric literals are preserved; no credentials,
price tokens or private session cookies are saved. Only Visa/Pix payment entries
are projected where present.

Angeloni used the existing catalog route with `count=10`, `page=1`, `sc=1`,
resolved `regionId`, and the adapter's public segment. Partitions were
`category-1/82/category-2/84` (beef),
`category-1/153/category-2/183` (fruit) and
`category-1/218/category-2/226` (bread). No keyword search was used.
Selected weight SKUs: `4721796` (kg/0.1, Price/ListPrice 49.49), `150177`
(kg/0.5, 6.59), `58297` (kg/0.1, 17.99); all `isKit=false`, no payment options.
The supplied public segment was not retained verbatim in the rendered session,
and the beef storefront was unavailable while the regional API exposed stock.
Some displayed `/ KG` prices matched, but the expected Bigorrilho context was
not independently established. That initially blocked rollout under an exact-region
comparison requirement. The accepted scope now permits another pickup location in
Curitiba. The collection location is established by the API's verified Bigorrilho
region; rendered `6.59 / KG` and `17.99 / KG` amounts establish advertised price
semantics. The storefront's unknown exact branch and unavailable beef do not
replace the API's seller, region or stock status. The mapping is now enabled for
the verified channel `1`, seller `1`, regional seller `superangeloni24`, resolved
Curitiba/PR API context. No unregionalized API fallback was introduced.

Festval used the existing target-search endpoint with `ft=banana`, `ft=patinho`
and `ft=pao`, `_from=0`, `_to=9`, `sc=2`, resolved `regionId` and public segment.
Selected weight SKUs: unavailable banana `7515` (kg/0.1000, Price 7.99),
beef `5568` (kg/0.5000, 76.98), approximate beef `5621` (kg/0.4000, 69.98),
and bread `2164` (kg/0.3440, 36.9), all `isKit=false`.
Beef/bread payment totals `3849`/`1269` minor units differ from the kg prices.
The rendered pages instead showed pickup at Carlos de Carvalho; the supplied
delivery segment was not retained verbatim. Banana had no main displayed price;
beef and bakery showed matching `/kg` amounts. The retailer's
[pickup information](https://www.festval.com/retirada-e-entrega) identifies Carlos
de Carvalho at Alameda Doutor Carlos de Carvalho, 1652, Bigorrilho, Curitiba.
This pickup view meets the accepted city-wide comparison scope. The mapping is
now enabled for channel `2`, seller `1`, regional seller `meufestvaldark`, the
`curitiba` marketing segment and a resolved Curitiba/PR API context. Observations
retain the actual API delivery context, independently of the display's pickup view.

The historical `*-candidates.json` filenames are retained; these are captured
source observations. Their verification files record the revised acceptance scope
and the exact-branch verification limitation. Parser tests assert unscaled kg
amounts, null packages, proper internal-code treatment, retained regional context
and independent fixed-package controls. Unverified cities, sellers and channels
remain unsupported. Festval's weight-payment portion totals are omitted with
issues; Angeloni's sampled kg responses expose no alternatives. Six opt-in live
checks cover each shop's produce, beef and bakery routes. No extra per-product
request, pickup substitution or browser dependency was added to collection.

`angeloni-size-policy.json` contains projections of five live catalog records retrieved
on 2026-10-30 for Curitiba/Bigorrilho (CEP 80730201, channel 1, regional seller
`superangeloni24`). It retains source names, identifiers, sale units, multipliers,
package properties and seller offers for the 60g BOLD bar, 45g Crisp Bar, ambiguous
Pringles listing, whole watermelon and ham with multiplier 200. Decimal source values
are stored as exact strings; no private credentials or account/session data is included.

`festval-size-policy.json` projects source responses inspected on 2026-10-30 in the
Curitiba delivery context (CEP 80730201, channel 2, regional seller `meufestvaldark`).
It covers six kilogram listings, Mentos 3L 8.5g and contradictory 250g/500g Vapza
beans. Source decimal quantities are exact strings; seller-offer projections retain
prices, stock and installment options without price tokens or session metadata.

## Hirota Nazaré pickup

`hirota-context.json`, `hirota-categories.json`, `hirota-products.json`,
`hirota-cereals.json`, `hirota-cleaning.json` and `hirota-coffee.json` are public
API captures from 2026-11-01. The context is organization 171, filial 1,
distribution center 1, pickup at Avenida Nazaré 1299, CEP 04263100, São Paulo/SP.
Source money remains exact JSON strings/numbers; no bearer tokens, session
headers, customer accounts, addresses or cookies are stored. The address in the
context fixture is the retailer's public branch address.

`hirota-verification.json` records capture times, public source/product URLs and
same-context rendered excerpts for rice, beans, milk, coffee, detergent, tomatoes
and potatoes. File timestamps recorded immediately after capture supply the
sample times; source capture and later rendered verification are distinguished.
`hirota-submission.json` is ingestion v1 output from those seven source records.
Go integration tests replay it on two **synthetic** local dates in disposable
PostgreSQL Testcontainers, verify worker processing and test searchable price
history, pickup provenance, retailer names, links and exact kg amounts.

Regular discounts retain their full source conditions and listing quantity limits.
Unknown payment/club/coupon terms are not unconditional offers. Kg offers use
explicit advertised kg prices, not rounded portion division; approximate portions
are not packages. The captured “1 Litro” spelling is recognized as fixed contents.
Internal short barcodes stay null. Quantity bands/family promotions remain
unsupported and produce issues. No unavailable listing was found in this sample;
unavailable, missing-price, quantity-condition and malformed cases in the Hirota
tests are explicitly synthetic mutations. Category fixtures retain source
pagination metadata; bounded pipeline tests deliberately substitute small totals.

See [onboarding evidence](../../../../docs/evidence/hirota-onboarding.md).

### Sonda context investigation

`sonda-context-check.json` is a sanitized live browser capture from 2026-11-01.
It records a successful MASP CEP serviceability check, empty selected-branch fields,
no anonymous pickup choices, and catalog price/identifier discrepancies. It is
research evidence, not an ingestion fixture or verified São Paulo price context.
See [the onboarding note](../../../../docs/plans/sonda-retailer.md).

### Pão de Açúcar / GPA

`pao-de-acucar-{delivery,branch}.json` are public HTTP responses resolving
CEP 04115090 to São Paulo Ricardo Jafet store 461 / modality 842. The adapter
requires both every run; an anonymous default store alone is insufficient.
`pao-de-acucar-{rice,coffee,cleaning,products}.json` preserve source category
listing bytes, including pagination, SKU strings, promotion flags and exact JSON
money. The generic products page includes melão SKU 0339087: R$14.29 estimated
portion versus an explicit R$12.99/kg. Only the latter becomes an offer.

`pao-de-acucar-verification.json` contains fresh rendered store-461 excerpts for
rice, beans, milk, coffee, detergent and melão, plus sanitized delivery state.
No customer address was selected in the browser: CEP serviceability is verified
independently through the public API, and prices are compared by explicit store
identity. `pao-de-acucar-submission.json` is adapter/shared-batcher output for
rice, coffee, detergent and kg melão, replayed on synthetic dates by disposable
PostgreSQL integration tests. Its observation timestamps are test timestamps.
No GTIN is provided by the listing API. Unavailable and quantity-promotion tests
are synthetic mutations, not captured claims. See the
[source evidence and limits](../../../../docs/evidence/pao-de-acucar-onboarding.md).

Sonda HTML excerpts and `sonda-verification.json` capture public source cards
and pagination. `sonda-submission.json` is ingestion v1 replay evidence for a
packaged Camil rice listing, using unknown fulfillment fields and online channel.
See [source provenance and limits](../../../../docs/plans/sonda-retailer.md).
