# Basic food basket monitoring scope

Migration 00006 uses the **DIEESE Pesquisa Nacional da Cesta Básica de Alimentos**
as the reference for food groups. Its 13 groups are listed below. Regional baskets
use different varieties and quantities; wheat/cassava flour and refined/crystal
sugar are separate monitoring targets. Potatoes are absent from the North and
Northeast reference baskets. See the
[DIEESE methodology, pages 2 and 5–6](https://www.dieese.org.br/metodologia/metodologiaCestaBasica2016.pdf)
and [Conab/DIEESE regional notes](https://www.gov.br/conab/pt-br/atuacao/informacoes-agropecuarias/precos-agropecuarios/analise-da-pesquisa-nacional-de-precos-da-cesta-basica-de-alimentos/analise-da-pesquisa-nacional-de-precos-da-cesta-basica-de-alimentos-todas-as-capitais-marco-2026.pdf).

| Reference food group | Monitoring target IDs |
| --- | --- |
| Beef | `carne-bovina-primeira` |
| Milk | `leite` |
| Beans | `feijao` |
| Rice | `arroz-branco-tipo-1` |
| Flour | `farinha-trigo`, `farinha-mandioca` |
| Potatoes | `batata` |
| Tomatoes | `tomate` |
| French bread | `pao-frances` |
| Ground coffee | `cafe-moido` |
| Bananas | `banana` |
| Sugar | `acucar-refinado`, `acucar-cristal` |
| Soybean oil | `oleo-soja` |
| Butter | `manteiga` |

There are **16 enabled targets**: 15 covering these groups and regional flour/sugar
variants, plus the previously monitored `macarrao` target. Existing target IDs and
rules are preserved. The broader liquid/powdered milk scope also remains in place.

For beef, discovery uses the methodology's boneless cuts: patinho, coxão mole
(chã de dentro), and coxão duro (chã de fora). Bread includes the regional search
names pão de sal and cacetinho. Fresh-produce rules exclude common processed
derivatives such as tomato sauce, potato chips and banana sweets. Butter excludes
margarine and peanut butter. These are name filters; listings can still lack
enough information for a match. [Shared matching examples](matching-examples.json)
cover accepted and rejected products for every seeded target.

All matching brands remain eligible. Each brand, cut, variety, processing type,
package size and GTIN retains its product identity. A monitoring group does not
authorize averaging its variants or substituting one for another.

## Definition and coverage limits

The government also defines a broader nutritional basket through ten food groups
in [Decree 11.936/2024, article 4](https://legis.senado.leg.br/norma/38236864/publicacao/38236971).
That policy definition is distinct from the 13-group price-monitoring reference
chosen here; this seed does not claim to enumerate its open-ended food list.

The targets are an independent collection scope inspired by the reference basket,
not a reproduction of DIEESE's sampling, regional quantities, brands or cost index.
No official weighting, basket total or inflation comparison is implemented here.
The retained milk scope and pasta are explicit differences from that reference.

Adding a target does not expand a shop adapter's supported price formats. The
Muffato, Angeloni, and Festval adapters currently handle individually sold
packaged goods (`measurementUnit=un`,
`unitMultiplier=1`). Variable-weight produce, meat and bakery listings remain
unsupported and generate collection issues when encountered. Missing/unsupported
coverage must not be presented as complete collection or a zero price. This
change registers those targets without guessing kilogram/package conversions.

Migration 00006 is edited in place as requested. Goose does not replay an already
applied version; an existing database that ran the earlier seed needs an explicit
forward data migration before it can expose the added targets.
