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
