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
