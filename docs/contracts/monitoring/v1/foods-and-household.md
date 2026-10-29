# Additional foods and household essentials

Migration **00008** adds **20 food targets and eight household targets**, expanding
the previous 37 to **65 enabled targets** on a fresh database. Goma de tapioca
and queijo minas frescal are intentionally outside these additions.

| Target ID | Product |
| --- | --- |
| `carne-bovina-acem` | Acém |
| `carne-bovina-musculo` | Músculo |
| `pernil-suino` | Pork leg |
| `lombo-suino` | Pork loin |
| `file-tilapia` | Tilapia fillets |
| `repolho` | Fresh cabbage |
| `beterraba` | Fresh beetroot |
| `chuchu` | Fresh chayote |
| `abobrinha` | Fresh zucchini |
| `mandioca` | Uncooked cassava/aipim/macaxeira, including frozen roots |
| `batata-doce` | Sweet potatoes |
| `mamao` | Fresh papaya |
| `limao` | Fresh limes/lemons |
| `melancia` | Fresh watermelon |
| `requeijao` | Requeijão |
| `queijo-prato` | Queijo prato |
| `vinagre` | Culinary vinegar |
| `extrato-tomate` | Tomato paste |
| `arroz-integral` | Brown rice |
| `arroz-parboilizado` | Non-brown parboiled rice |
| `papel-higienico` | Dry toilet paper |
| `sabonete-barra` | Bar soap |
| `creme-dental` | Toothpaste |
| `absorvente` | External menstrual pads |
| `detergente-louca` | Manual dishwashing detergent |
| `sabao-roupas-po` | Laundry powder |
| `sabao-roupas-liquido` | Liquid laundry detergent |
| `agua-sanitaria` | Plain liquid bleach |

Brown parboiled rice belongs to `arroz-integral`; it is excluded from the second
rice target. Processed meat dishes, juice, cosmetic fragrances, cleaning vinegar,
prepared root dishes, cheese mixtures and inappropriate household variants have
explicit exclusions in the [shared Go/Python examples](matching-examples.json).
External pads exclude tampons, incontinence products and nursing pads. Bar soap
excludes explicitly liquid/intimate soap; dish detergent excludes machine products;
laundry liquid requires explicit clothing and liquid terms. Ambiguous names can
remain unmatched. Every food cut, variety, brand and package retains its identity.

The migration also appends household exclusions to the existing apple, orange,
butter and soybean-oil rules, preventing matches such as apple detergent and soap
with shea butter or soybean oil. It preserves their other criteria, names and
enabled flags. Existing history and catalog identities remain unchanged.

These targets are an independent collection scope. Household essentials are not
part of the food-basket reference, and no combined inflation index or weighting
is established by collecting them.

## Activation and category coverage

Restart the updated core to apply pending migrations through 00008. All adapters
fetch the expanded scope on their next run, including the daily scheduler.

Adding targets is a core-only change with the default scraper configuration.
Muffato and Festval use core-provided search queries. Angeloni automatically
discovers all available leaf categories and filters the catalog using core rules.
There is no product-specific category list to maintain.

When upgrading, remove `catalog_category_ids` from older Angeloni configurations
once. The example configurations already omit it. An explicit category list is
still accepted for bounded diagnostics, but restricts coverage and can prevent
new targets from being found. Automatic browsing retains the existing per-category
and overall page budgets; budget exhaustion reports incomplete collection.
Restart an existing scheduler once to load this adapter change. Future core
target additions are discovered at the next run without restarting the scraper.

Paths were checked against Angeloni's public
[`category/tree/5` endpoint](https://super.angeloni.com.br/api/catalog_system/pub/category/tree/5)
and saved in the [category fixture](../../../../scrapers/tests/fixtures/stores/README.md).
Category navigation does not establish live stock, successful price extraction,
or complete coverage for every target. Existing regional, seller, price-basis,
pagination and per-target coverage checks continue to apply.

## Household package limits

Source names preserve available roll counts, sheet types, roll lengths, pad counts
and concentration descriptions. The current structured package contract supports
grams, kilograms, millilitres, litres and units. It has no fields for sheet layers
or roll length, and generic multipack/count extraction is not implemented.
Unverified or ambiguous quantities remain null; household pack counts are not
inferred from the egg-specific count mapping. No price-per-roll, price-per-metre,
price-per-pad or price-per-wash comparison is calculated.

Single-pack soap/toothpaste weights and detergent/bleach volumes use the existing
verified name-size rules where the source provides clear declarations. Review
actual listings and coverage after the first expanded collection run.
