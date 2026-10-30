# Family essentials monitoring additions

Migration **00009** adds **ten household essentials and two raw chicken groups**,
bringing the configured scope from 65 to 77 targets at that version. Migration
00010 adds [19 snacks, sweets and convenience foods](snacks-and-convenience.md),
bringing the current fresh-database scope to **96 targets**.
Infant formula and cooking gas are outside this expansion.

| Target ID | Product |
| --- | --- |
| `fralda-infantil` | Disposable baby diapers |
| `fralda-adulta` | Disposable adult diapers |
| `escova-dental` | Manual toothbrushes, including children's brushes |
| `shampoo` | Human-use shampoo, including children's shampoo |
| `sabao-roupas-barra` | Bar laundry soap |
| `saco-lixo` | Rubbish bags |
| `esponja-louca` | Dishwashing sponges |
| `desodorante` | Body deodorant |
| `fio-dental` | Dental floss and dental tape |
| `frango-inteiro` | Raw whole chicken, chilled or frozen |
| `frango-coxa-sobrecoxa` | Raw chicken drumstick/thigh combined packages or attached cuts |
| `desinfetante` | Liquid household disinfectant |

The [shared Go/Python matching examples](matching-examples.json) cover accepted
names and exclusions for all 96 groups. Baby diapers exclude explicit adult,
geriatric and incontinence descriptions; adult diapers require those descriptors.
Reusable cloth diapers, swimming diapers and pet products are outside the baby
group. Names without adult descriptors can still be ambiguous: these are name
filters, not a verified age classification. Preserve original size and variant
descriptions; adult and baby sizes are not interchangeable.

Toothbrushes exclude electric brushes, replacement heads and toothpaste kits.
Shampoo excludes pet/automotive products and explicit conditioner combinations.
Bar laundry soap excludes body/hand soap. Sponges require dishwashing, multiuse,
double-face or non-scratch wording and exclude steel wool, bath, makeup and
melamine products. Deodorants exclude room, toilet and foot products. Floss
excludes holders, picks and kits. Disinfectants exclude aerosol/spray, gel,
wipe, hospital, veterinary and laundry products.

Whole chicken excludes cuts, cooked, seasoned and stuffed products. Combined
drumstick/thigh listings require both cut names and remain excluded from the two
individual-cut groups. Each brand, size, raw/chilled/frozen variant and GTIN keeps
its own identity. No substitutions or combined inflation measure are implied.

The migration also excludes shampoo/xampu and deodorant names from food groups
whose ingredients commonly appear in cosmetics. Soap/disinfectant exclusions
are added to banana, milk, eggs, carrot and oat targets. Existing target names,
enabled flags, other rules and price histories are preserved. Existing catalog
associations are not reclassified or backfilled.

## Activation and collection limits

Rebuild and restart the updated core to apply migration 00009. All three adapters
fetch the expanded scope at their next run. Muffato and Festval search the supplied
queries; Angeloni discovers live leaf categories automatically. Older explicit
`catalog_category_ids` restrictions must be removed to enable that discovery.
No product-specific configuration changes are required with the default settings.

Update and restart running scrapers once to load the package-parsing guard for
diapers and rubbish bags. This keeps diaper wearer-weight ranges and bag litre
capacities from being stored as package net contents. Future target additions
continue to be discovered automatically from the core.

New targets do not establish live stock or complete coverage. Review missing
targets, parser issues and per-target counts after collection. Existing seller,
regional, request-budget, kit and price-basis checks still apply.

The [household package limits](foods-and-household.md#household-package-limits)
also apply here. Generic diaper/brush/sponge/bag counts and floss length are not
new structured fields or inferred quantities. These descriptions remain in source
names; unknown or unsupported quantities stay null. No per-diaper, per-bag,
per-metre or per-use comparison is introduced.
