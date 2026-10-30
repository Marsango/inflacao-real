# Snacks, sweets and convenience foods

Migration **00010** adds **19 targets**, bringing the configured scope from 77 to
**96 groups** on a fresh database. Fruit nectar and powdered drink mixes are not
included. Refined and crystal sugar remain covered by the existing targets;
infant formula and cooking gas remain outside this expansion.

| Target ID | Product |
| --- | --- |
| `refrigerante` | Soft drinks, including regular and zero-sugar variants |
| `chocolate-barra` | Chocolate bars and tablets |
| `bombom` | Bombons, individually sold or assorted boxes |
| `achocolatado-po` | Powdered sweetened chocolate drink mix |
| `bala` | Candies, including gummy candies |
| `chiclete` | Chewing gum |
| `biscoito-doce` | Sweet biscuits and cookies without filling |
| `biscoito-recheado` | Filled biscuits and cookies |
| `biscoito-wafer` | Wafer biscuits |
| `salgadinho` | Packaged savory snacks, excluding potato chips |
| `batata-chips` | Packaged potato chips, excluding potato sticks and frozen fries |
| `milho-pipoca` | Plain popcorn kernels |
| `sorvete` | Ice cream |
| `leite-condensado` | Condensed milk |
| `macarrao-instantaneo` | Instant noodles |
| `salsicha` | Salsichas, including hot-dog sausages |
| `presunto` | Ham |
| `mortadela` | Mortadella |
| `linguica` | Linguiça, including fresh, smoked and cured varieties |

The [shared Go/Python examples](matching-examples.json) cover accepted names and
rejected neighboring products. These remain deterministic name filters: absent
descriptors can leave a listing unmatched, and a name alone cannot establish a
verified product taxonomy.

Chocolate bars exclude bombons, cocoa/drink powders, baking coatings, Easter eggs,
biscuits and ice cream. Chocolate listings without a bar/tablet descriptor remain
eligible when none of the exclusions applies. Wafer biscuits are separate from
other filled biscuits; plain sweet biscuits exclude both. Savory snacks exclude
potato chips, fruit/nut snacks, frozen party snacks and pet food. Popcorn kernels
exclude microwave bags, ready-to-eat and sweetened popcorn.

Powdered chocolate drink mix requires explicit powder wording and excludes
ready-to-drink beverages and plain cocoa/chocolate powders. Condensed milk
excludes explicit dairy mixtures and desserts flavored with condensed milk.
Chewing gum excludes nicotine products. Ham excludes apresuntado and turkey
breast substitutes. The meat targets exclude sandwiches, pizzas, pâtés and other
prepared dishes; fresh, cooked, smoked, cured and species variants retain their
own source names and product identities.

Ingredient groups exclude flavored sweets, desserts and processed meats where
names otherwise overlap. For example, banana candies do not match fresh bananas,
onion-flavored chips do not match onions, and chicken-breast linguiça does not
match raw chicken breast. Existing names, enabled settings, other rules and price
history are preserved. Associations remain additive; this does not reclassify
existing products or backfill their target associations.

## Activation and coverage

Rebuild and restart the core to apply migration 00010. Scrapers fetch the expanded
scope at their next run with the default configuration. Muffato/Festval search the
core queries; Angeloni discovers live leaf categories and filters the catalog.
No new adapter code or category configuration is required. Older explicit
Angeloni category restrictions must be removed to enable automatic discovery.

Regular/zero-sugar drinks, chocolate types, flavors, brands, package sizes,
fresh/frozen states, meat species and curing types remain separate products.
Groups do not authorize averaging or substituting these variants. No inflation
index or expenditure weights are introduced.

Existing package parsing, multipack, kit, seller, regional and price-format limits
continue to apply. Deli meat prices advertised per kg preserve that price basis;
approximate portions do not become fixed packages. Unknown quantities stay null,
and box/pack counts are not newly inferred. Missing stock, blocked pages, request
budgets and unsupported formats remain explicit coverage limits. Review per-target
counts and collection issues after the first expanded run; seeded groups alone
do not demonstrate live retailer coverage.
