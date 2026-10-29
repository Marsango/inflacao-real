# Remaining weight and package work

Kilogram mappings are enabled for Muffato's Champagnat online catalog (`sc=8`,
seller `1`, no selected CEP), Angeloni's Bigorrilho delivery catalog and Festval's
Curitiba delivery catalog. Produce, beef and bakery samples are documented in
[fixture provenance](tests/fixtures/README.md) and
[regional verification evidence](tests/fixtures/stores/README.md).

- Expand verified Curitiba shop/seller configurations when needed. Comparisons may
  use another Curitiba pickup location; each observation retains the actual API
  seller, CEP, region and availability. Exact storefront-branch stock parity has
  not been established and is not required for the city-wide collection scope.
- Establish representable weight-payment alternatives. Current portion totals
  remain omitted with issues; valid base kg observations still get delivered.
- Capture complete SKU-specific net contents and verified homogeneous multipacks
  before adding extraction paths. No such structured fields were established in
  this sample. Unknown/ambiguous contents stay null; there is no extra detail fetch.
- Verify further price/portion formats, fractional purchase minima and promotion
  thresholds independently; sale increments do not establish those conditions.
- Verify a configured running core and two fresh real collection runs separately
  from parser smoke checks and disposable-database integration tests.

Tests establish retention of current sizes, component counts and nulls. Automatic
shrinkflation detection, equivalent-product replacement matching, unit-price
calculations, basket totals and inflation methodology remain core work.
