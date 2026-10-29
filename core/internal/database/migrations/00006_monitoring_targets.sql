-- +goose Up
CREATE TABLE monitoring_targets (
    id text PRIMARY KEY CHECK (id ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    name text NOT NULL CHECK (length(trim(name)) > 0),
    criteria jsonb NOT NULL CHECK (jsonb_typeof(criteria) = 'object'),
    enabled boolean NOT NULL DEFAULT true
);

-- Brand-independent discovery rules, not canonical products or an inflation basket.
-- Package sizes and brands remain separate canonical variants during ingestion.
-- Scope reference: DIEESE basic food basket, with regional flour/sugar variants.
-- https://www.dieese.org.br/metodologia/metodologiaCestaBasica2016.pdf
-- See docs/contracts/monitoring/v1/basic-food-basket.md for sources and scope limits.
-- Pasta and the broader milk scope are retained additions to the reference basket.
INSERT INTO monitoring_targets (id, name, criteria) VALUES
('arroz-branco-tipo-1', 'Arroz branco tipo 1', '{"search_queries":["arroz"],"required_terms":[["arroz"],["branco","polido"],["tipo 1","tipo1"]],"excluded_terms":["integral","parboilizado","arboreo","carreteiro","pronto","farinha","biscoito"]}'),
('feijao', 'Feijão em grãos', '{"search_queries":["feijao"],"required_terms":[["feijao"]],"excluded_terms":["pronto","temperado","caldo de feijao","farinha","sopa"]}'),
('leite', 'Leite líquido ou em pó', '{"search_queries":["leite"],"required_terms":[["leite"]],"excluded_terms":["condensado","creme","doce","coco","bebida","composto","chocolate","biscoito","sabonete"]}'),
('cafe-moido', 'Café torrado e moído', '{"search_queries":["cafe"],"required_terms":[["cafe"],["moido","po"]],"excluded_terms":["soluvel","capsula","capsulas","graos","bebida","filtro","cafeteira","leite"]}'),
('acucar-refinado', 'Açúcar refinado', '{"search_queries":["acucar"],"required_terms":[["acucar"],["refinado"]],"excluded_terms":["confeiteiro","mistura"]}'),
('oleo-soja', 'Óleo de soja', '{"search_queries":["oleo soja"],"required_terms":[["oleo"],["soja"]],"excluded_terms":["composto","mistura"]}'),
('macarrao', 'Macarrão seco', '{"search_queries":["macarrao"],"required_terms":[["macarrao"]],"excluded_terms":["instantaneo","pronto","congelado"]}'),
('carne-bovina-primeira', 'Carne bovina de primeira: patinho e coxão', '{"search_queries":["patinho","coxao mole","coxao duro","cha de dentro","cha de fora"],"required_terms":[["patinho","coxao mole","coxao duro","cha de dentro","cha de fora"]],"excluded_terms":["suino","suina","porco","frango","hamburguer","almondega","almondegas","empanado","temperado","temperada","cozido","cozida","pronto","sopa","racao","com osso"]}'),
('farinha-trigo', 'Farinha de trigo', '{"search_queries":["farinha trigo"],"required_terms":[["farinha"],["trigo"]],"excluded_terms":["integral","mistura","farofa","rosca","lactea"]}'),
('farinha-mandioca', 'Farinha de mandioca', '{"search_queries":["farinha mandioca"],"required_terms":[["farinha"],["mandioca"]],"excluded_terms":["farofa","temperada","temperado","mistura","tapioca"]}'),
('batata', 'Batata fresca', '{"search_queries":["batata"],"required_terms":[["batata"]],"excluded_terms":["doce","frita","fritas","chips","palha","congelada","congeladas","congelado","congelados","pure","amido","farinha","nhoque","gnocchi","pao","salgadinho","racao","snack","semente","sementes"]}'),
('tomate', 'Tomate fresco', '{"search_queries":["tomate"],"required_terms":[["tomate"]],"excluded_terms":["molho","extrato","ketchup","catchup","seco","secos","desidratado","pelado","pelados","conserva","suco","sopa","polpa","salada","tempero","sabor","semente","sementes"]}'),
('pao-frances', 'Pão francês ou pão de sal', '{"search_queries":["pao frances","pao de sal","cacetinho"],"required_terms":[["pao frances","pao tipo frances","pao de sal","cacetinho"]],"excluded_terms":["torrada","torradas","farinha","forma","alho","recheado","integral","doce","hamburguer","congelado","congelados","congelada"]}'),
('banana', 'Banana fresca', '{"search_queries":["banana"],"required_terms":[["banana"]],"excluded_terms":["chips","frita","fritas","desidratada","passa","doce","bala","bolo","farinha","vitamina","suco","pure","geleia","bananada"]}'),
('manteiga', 'Manteiga', '{"search_queries":["manteiga"],"required_terms":[["manteiga"]],"excluded_terms":["margarina","creme vegetal","amendoim","cacau","coco","sabor","biscoito","bolacha","garrafa","ghee","pipoca"]}'),
('acucar-cristal', 'Açúcar cristal', '{"search_queries":["acucar cristal"],"required_terms":[["acucar"],["cristal"]],"excluded_terms":["confeiteiro","mistura","demerara","mascavo"]}');

CREATE TABLE product_monitoring_targets (
    product_id bigint NOT NULL REFERENCES products(id),
    target_id text NOT NULL REFERENCES monitoring_targets(id),
    PRIMARY KEY (product_id, target_id)
);
CREATE INDEX product_monitoring_targets_target_idx ON product_monitoring_targets (target_id, product_id);

-- +goose Down
DROP TABLE product_monitoring_targets;
DROP TABLE monitoring_targets;
