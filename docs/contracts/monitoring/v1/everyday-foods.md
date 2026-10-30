# Everyday food monitoring additions

Migration **00007** adds these 21 targets to the original 16, for **37 targets**
at that version. Migration 00008 adds [more foods and household essentials](foods-and-household.md),
bringing the scope to 65 targets at that version. Migration 00009 adds
[12 family essentials](family-essentials.md), for 77 groups at that version.
Migration 00010 adds [19 snacks, sweets and convenience foods](snacks-and-convenience.md),
bringing the current fresh-database scope to **96 targets**.
These additions expand the independent collection
scope beyond the [DIEESE-inspired reference](basic-food-basket.md). They do not
define a weighted inflation basket or establish national consumption rankings.

| Target ID | Food |
| --- | --- |
| `ovos` | Chicken eggs |
| `frango-peito` | Chicken breast |
| `frango-coxa` | Chicken drumsticks |
| `frango-sobrecoxa` | Chicken thighs |
| `cebola` | Fresh onions |
| `alho` | Fresh garlic |
| `sal-refinado` | Refined salt |
| `cenoura` | Fresh carrots |
| `alface` | Fresh lettuce |
| `laranja` | Fresh oranges |
| `molho-tomate` | Tomato sauce |
| `fuba` | Cornmeal |
| `flocao-milho` | Flaked corn for cuscuz |
| `queijo-mucarela` | Mozzarella |
| `maca` | Fresh apples |
| `pao-forma` | Sliced sandwich bread |
| `iogurte` | Yogurt |
| `aveia` | Oat flakes, bran, or flour |
| `margarina` | Margarine |
| `sardinha-conserva` | Canned/preserved sardines |
| `biscoito-salgado` | Savory crackers |

Every target has positive and negative [shared matching examples](matching-examples.json)
executed by Go and Python. Rules require explicit identifying words and exclude
common derivatives: orange juice does not match fresh oranges, prepared chicken
does not match raw cuts, and sardines in tomato sauce do not match tomato sauce.
Ambiguous names can remain unmatched. These deterministic name rules need
refinement as actual source listings establish omissions or false positives.

Chicken cuts have separate targets; mixed drumstick/thigh packages are excluded
from both. Fubá and flocão remain separate. Cheese rules exclude buffalo and
mixed/imitation cheese. Yogurt excludes explicitly labeled dairy drinks; oats
exclude prepared cereal bars, biscuits, and beverages. The existing identity
rules continue to preserve brands, varieties, fresh/frozen condition, flavors,
and package sizes. Membership and payment prices retain their conditions.

## Activation and adapter coverage

Restart the core with the updated code (or rebuild and restart its binary).
Startup applies pending migrations, including 00007, to existing databases. This migration adds the targets
without editing migration 00006 or rewriting existing target settings or history.
Each scraper fetches the expanded scope at its next collection run, including
runs started by the daily scheduler.

Muffato and Festval use the new targets' search queries. Angeloni browses category
partitions and filters the same scope locally. It now discovers all live leaf
categories automatically. Remove `catalog_category_ids` from older configurations
once; new products and categories then require no scraper changes. Explicit
category lists remain available only as optional diagnostic restrictions.

The IDs and paths were checked against Angeloni's public
[`category/tree/5` endpoint](https://super.angeloni.com.br/api/catalog_system/pub/category/tree/5).
The projected tree is saved with [fixture provenance](../../../../scrapers/tests/fixtures/stores/README.md).
This verifies category navigation, not live availability or collection success
for every new food. Existing price-format, package, seller, and regional checks
continue to apply; missing targets and unsupported listings remain explicit
coverage failures. Review per-target counts after the first expanded run.
