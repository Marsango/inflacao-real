# Regional-store response fixtures

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
  response. Keeps paths for the 15 configured basket-related leaf categories and
  one chocolate category used in pipeline tests. Names, IDs, parentage and
  `hasChildren` flags retain their source values.
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
