-- +goose Up
-- Add nineteen everyday treats, snacks and convenience foods.
INSERT INTO monitoring_targets (id, name, criteria) VALUES
('refrigerante', 'Refrigerantes', '{"search_queries":["refrigerante"],"required_terms":[["refrigerante","refrigerantes"]],"excluded_terms":["xarope","concentrado","po","bala","balas","goma","sorvete","kit"]}'),
('chocolate-barra', 'Chocolate em barra ou tablete', '{"search_queries":["chocolate","chocolate barra"],"required_terms":[["chocolate","chocolates"]],"excluded_terms":["bombom","bombons","trufa","trufas","achocolatado","po","bebida","cereal","biscoito","biscoitos","bolacha","bolachas","cookie","cookies","wafer","sorvete","picole","ovo","ovos","pascoa","cobertura","fracionado","fracionada","gota","gotas","granulado","creme","pasta","bolo","pao","leite condensado","iogurte","mistura","sabonete","shampoo","kit"]}'),
('bombom', 'Bombons', '{"search_queries":["bombom","bombons"],"required_terms":[["bombom","bombons"]],"excluded_terms":["sorvete","picole","bolo","biscoito","bolacha","wafer","pave","torta","iogurte","essencia","sabor bombom","kit"]}'),
('achocolatado-po', 'Achocolatado em pó', '{"search_queries":["achocolatado em po"],"required_terms":[["achocolatado","achocolatados"],["po"]],"excluded_terms":["liquido","liquida","pronto","pronta","bebida","biscoito","bolacha","bolo","kit"]}'),
('bala', 'Balas e gomas doces', '{"search_queries":["bala","balas"],"required_terms":[["bala","balas","jujuba","jujubas"]],"excluded_terms":["chiclete","chicletes","goma de mascar","gomas de mascar","pirulito","vitamina","vitaminas","suplemento","medicinal","sorvete","picole","kit"]}'),
('chiclete', 'Chicletes e gomas de mascar', '{"search_queries":["chiclete","goma de mascar"],"required_terms":[["chiclete","chicletes","chicle","chicles","goma de mascar","gomas de mascar"]],"excluded_terms":["nicotina","medicinal","suplemento","sorvete","picole","bala","balas","kit"]}'),
('biscoito-doce', 'Biscoitos doces sem recheio', '{"search_queries":["biscoito doce","biscoito maisena","biscoito maria","cookie"],"required_terms":[["biscoito","biscoitos","bolacha","bolachas","cookie","cookies"],["doce","doces","maisena","maizena","maria","leite","coco","amanteigado","amanteigados","chocolate","cookie","cookies"]],"excluded_terms":["recheado","recheados","recheada","recheadas","wafer","wafers","salgado","salgados","cracker","crackers","agua e sal","racao","pet","kit"]}'),
('biscoito-recheado', 'Biscoitos recheados', '{"search_queries":["biscoito recheado","bolacha recheada"],"required_terms":[["biscoito","biscoitos","bolacha","bolachas","cookie","cookies"],["recheado","recheados","recheada","recheadas"]],"excluded_terms":["wafer","wafers","salgado","salgados","racao","pet","kit"]}'),
('biscoito-wafer', 'Biscoitos wafer', '{"search_queries":["biscoito wafer","wafer"],"required_terms":[["wafer","wafers"]],"excluded_terms":["sorvete","picole","bombom","bombons","casquinha","cone","bolo","kit"]}'),
('salgadinho', 'Salgadinhos de pacote', '{"search_queries":["salgadinho","snack salgado"],"required_terms":[["salgadinho","salgadinhos","snack","snacks"]],"excluded_terms":["batata","banana","fruta","frutas","doce","doces","chocolate","biscoito","bolacha","amendoim","castanha","racao","pet","cachorro","gato","congelado","congelados","coxinha","risole","kit"]}'),
('batata-chips', 'Batata chips de pacote', '{"search_queries":["batata chips","batata frita"],"required_terms":[["batata","batatas"],["chips","frita","fritas","ondulada","onduladas"]],"excluded_terms":["palha","congelada","congeladas","congelado","congelados","pure","farinha","massa","doce","racao","kit"]}'),
('milho-pipoca', 'Milho para pipoca em grãos', '{"search_queries":["milho pipoca"],"required_terms":[["milho"],["pipoca"]],"excluded_terms":["microondas","micro ondas","pronta","pronto","doce","caramelizada","caramelizado","manteiga","oleo","semente","sementes","plantio","racao","kit"]}'),
('sorvete', 'Sorvetes', '{"search_queries":["sorvete"],"required_terms":[["sorvete","sorvetes"]],"excluded_terms":["mistura","preparo","po","cobertura","calda","casquinha","cone","base","kit"]}'),
('leite-condensado', 'Leite condensado', '{"search_queries":["leite condensado"],"required_terms":[["leite"],["condensado"]],"excluded_terms":["mistura","composto","bebida","sorvete","picole","bolo","biscoito","bolacha","bala","balas","bombom","bombons","sabor","kit"]}'),
('macarrao-instantaneo', 'Macarrão instantâneo', '{"search_queries":["macarrao instantaneo","lamen"],"required_terms":[["macarrao","massa","massas","lamen","noodles"],["instantaneo","instantanea","instantaneos","instantaneas"]],"excluded_terms":["tempero","molho","sopa","biscoito","racao","kit"]}'),
('salsicha', 'Salsichas', '{"search_queries":["salsicha"],"required_terms":[["salsicha","salsichas"]],"excluded_terms":["pizza","sanduiche","sanduiches","pao","lasanha","farofa","feijoada","pate","racao","salgadinho","bolo","torta","tortas","molho","macarrao","risoto","kit","linguica"]}'),
('presunto', 'Presunto', '{"search_queries":["presunto"],"required_terms":[["presunto"]],"excluded_terms":["pizza","sanduiche","sanduiches","pao","lasanha","farofa","feijoada","pate","racao","salgadinho","bolo","torta","tortas","molho","macarrao","risoto","kit","apresuntado","peito de peru"]}'),
('mortadela', 'Mortadela', '{"search_queries":["mortadela"],"required_terms":[["mortadela"]],"excluded_terms":["pizza","sanduiche","sanduiches","pao","lasanha","farofa","feijoada","pate","racao","salgadinho","bolo","torta","tortas","molho","macarrao","risoto","kit"]}'),
('linguica', 'Linguiça', '{"search_queries":["linguica"],"required_terms":[["linguica","linguicas"]],"excluded_terms":["pizza","sanduiche","sanduiches","pao","lasanha","farofa","feijoada","pate","racao","salgadinho","bolo","torta","tortas","molho","macarrao","risoto","kit"]}');

-- Keep flavored sweets and prepared products out of ingredient/raw-cut groups.
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') || '["balas", "bombom", "bombons", "chiclete", "chicletes", "chicle", "chicles", "sorvete", "sorvetes", "picole", "picoles", "biscoito", "biscoitos", "bolacha", "bolachas", "wafer", "chocolate", "refrigerante", "refrigerantes", "salgadinho", "cookie", "cookies", "achocolatado", "jujuba", "jujubas"]'::jsonb)
WHERE id='banana';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') || '["bala", "balas", "bombom", "bombons", "chiclete", "chicletes", "chicle", "chicles", "sorvete", "sorvetes", "picole", "picoles", "biscoitos", "bolachas", "wafer", "chocolate", "refrigerante", "refrigerantes", "salgadinho", "cookie", "cookies", "achocolatado", "jujuba", "jujubas"]'::jsonb)
WHERE id='maca';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') || '["balas", "bombom", "bombons", "chiclete", "chicletes", "chicle", "chicles", "sorvete", "sorvetes", "picole", "picoles", "biscoitos", "bolachas", "wafer", "chocolate", "refrigerantes", "salgadinho", "cookie", "cookies", "achocolatado", "jujuba", "jujubas"]'::jsonb)
WHERE id='laranja';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') || '["balas", "bombom", "bombons", "chiclete", "chicletes", "chicle", "chicles", "sorvete", "sorvetes", "picole", "picoles", "biscoitos", "bolachas", "wafer", "chocolate", "refrigerantes", "salgadinho", "cookie", "cookies", "achocolatado", "jujuba", "jujubas"]'::jsonb)
WHERE id='mamao';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') || '["balas", "bombom", "bombons", "chiclete", "chicletes", "chicle", "chicles", "sorvete", "sorvetes", "picole", "picoles", "biscoitos", "bolachas", "wafer", "chocolate", "refrigerantes", "salgadinho", "cookie", "cookies", "achocolatado", "jujuba", "jujubas"]'::jsonb)
WHERE id='limao';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') || '["balas", "bombom", "bombons", "chiclete", "chicletes", "chicle", "chicles", "sorvete", "sorvetes", "picole", "picoles", "biscoitos", "bolachas", "wafer", "chocolate", "refrigerantes", "salgadinho", "cookie", "cookies", "achocolatado", "jujuba", "jujubas"]'::jsonb)
WHERE id='melancia';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') || '["bala", "balas", "bombom", "bombons", "chiclete", "chicletes", "chicle", "chicles", "sorvete", "sorvetes", "picole", "picoles", "biscoitos", "bolacha", "bolachas", "wafer", "refrigerante", "refrigerantes", "salgadinho", "cookie", "cookies", "achocolatado", "jujuba", "jujubas"]'::jsonb)
WHERE id='leite';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') || '["bala", "balas", "bombom", "bombons", "chiclete", "chicletes", "chicle", "chicles", "sorvete", "sorvetes", "picole", "picoles", "biscoitos", "bolachas", "wafer", "chocolate", "refrigerante", "refrigerantes", "salgadinho", "cookie", "cookies", "achocolatado", "jujuba", "jujubas"]'::jsonb)
WHERE id='aveia';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') || '["bala", "balas", "bombom", "bombons", "chiclete", "chicletes", "chicle", "chicles", "sorvete", "sorvetes", "picole", "picoles", "biscoitos", "bolachas", "wafer", "chocolate", "refrigerante", "refrigerantes", "salgadinho", "cookie", "cookies", "achocolatado", "jujuba", "jujubas"]'::jsonb)
WHERE id='manteiga';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') || '["bala", "balas", "bombom", "bombons", "chiclete", "chicletes", "chicle", "chicles", "sorvete", "sorvetes", "picole", "picoles", "biscoito", "biscoitos", "bolacha", "bolachas", "wafer", "chocolate", "refrigerante", "refrigerantes", "salgadinho", "cookie", "cookies", "achocolatado", "jujuba", "jujubas"]'::jsonb)
WHERE id='cenoura';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') || '["linguica", "salsicha", "presunto", "mortadela"]'::jsonb)
WHERE id='carne-bovina-primeira';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') || '["linguica", "salsicha", "presunto", "mortadela"]'::jsonb)
WHERE id='carne-bovina-acem';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') || '["linguica", "salsicha", "presunto", "mortadela"]'::jsonb)
WHERE id='carne-bovina-musculo';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') || '["linguica", "salsicha", "presunto", "mortadela"]'::jsonb)
WHERE id='pernil-suino';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') || '["linguica", "salsicha", "presunto", "mortadela"]'::jsonb)
WHERE id='lombo-suino';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') || '["linguica", "salsicha", "presunto", "mortadela"]'::jsonb)
WHERE id='frango-peito';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') || '["linguica", "salsicha", "presunto", "mortadela"]'::jsonb)
WHERE id='frango-coxa';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') || '["linguica", "salsicha", "presunto", "mortadela"]'::jsonb)
WHERE id='frango-sobrecoxa';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') || '["linguica", "salsicha", "presunto", "mortadela"]'::jsonb)
WHERE id='frango-inteiro';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') || '["linguica", "salsicha", "presunto", "mortadela"]'::jsonb)
WHERE id='frango-coxa-sobrecoxa';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') || '["instantanea", "instantaneos", "instantaneas"]'::jsonb)
WHERE id='macarrao';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') || '["macarrao", "lamen", "instantaneo", "instantanea"]'::jsonb)
WHERE id='feijao';

UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') || '["chips","snack","snacks","sabor","fritas"]'::jsonb)
WHERE id IN ('cebola','alho');

-- +goose Down
DELETE FROM product_monitoring_targets WHERE target_id IN ('refrigerante', 'chocolate-barra', 'bombom', 'achocolatado-po', 'bala', 'chiclete', 'biscoito-doce', 'biscoito-recheado', 'biscoito-wafer', 'salgadinho', 'batata-chips', 'milho-pipoca', 'sorvete', 'leite-condensado', 'macarrao-instantaneo', 'salsicha', 'presunto', 'mortadela', 'linguica');
DELETE FROM monitoring_targets WHERE id IN ('refrigerante', 'chocolate-barra', 'bombom', 'achocolatado-po', 'bala', 'chiclete', 'biscoito-doce', 'biscoito-recheado', 'biscoito-wafer', 'salgadinho', 'batata-chips', 'milho-pipoca', 'sorvete', 'leite-condensado', 'macarrao-instantaneo', 'salsicha', 'presunto', 'mortadela', 'linguica');
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') - ARRAY['balas', 'bombom', 'bombons', 'chiclete', 'chicletes', 'chicle', 'chicles', 'sorvete', 'sorvetes', 'picole', 'picoles', 'biscoito', 'biscoitos', 'bolacha', 'bolachas', 'wafer', 'chocolate', 'refrigerante', 'refrigerantes', 'salgadinho', 'cookie', 'cookies', 'achocolatado', 'jujuba', 'jujubas'])
WHERE id='banana';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') - ARRAY['bala', 'balas', 'bombom', 'bombons', 'chiclete', 'chicletes', 'chicle', 'chicles', 'sorvete', 'sorvetes', 'picole', 'picoles', 'biscoitos', 'bolachas', 'wafer', 'chocolate', 'refrigerante', 'refrigerantes', 'salgadinho', 'cookie', 'cookies', 'achocolatado', 'jujuba', 'jujubas'])
WHERE id='maca';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') - ARRAY['balas', 'bombom', 'bombons', 'chiclete', 'chicletes', 'chicle', 'chicles', 'sorvete', 'sorvetes', 'picole', 'picoles', 'biscoitos', 'bolachas', 'wafer', 'chocolate', 'refrigerantes', 'salgadinho', 'cookie', 'cookies', 'achocolatado', 'jujuba', 'jujubas'])
WHERE id='laranja';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') - ARRAY['balas', 'bombom', 'bombons', 'chiclete', 'chicletes', 'chicle', 'chicles', 'sorvete', 'sorvetes', 'picole', 'picoles', 'biscoitos', 'bolachas', 'wafer', 'chocolate', 'refrigerantes', 'salgadinho', 'cookie', 'cookies', 'achocolatado', 'jujuba', 'jujubas'])
WHERE id='mamao';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') - ARRAY['balas', 'bombom', 'bombons', 'chiclete', 'chicletes', 'chicle', 'chicles', 'sorvete', 'sorvetes', 'picole', 'picoles', 'biscoitos', 'bolachas', 'wafer', 'chocolate', 'refrigerantes', 'salgadinho', 'cookie', 'cookies', 'achocolatado', 'jujuba', 'jujubas'])
WHERE id='limao';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') - ARRAY['balas', 'bombom', 'bombons', 'chiclete', 'chicletes', 'chicle', 'chicles', 'sorvete', 'sorvetes', 'picole', 'picoles', 'biscoitos', 'bolachas', 'wafer', 'chocolate', 'refrigerantes', 'salgadinho', 'cookie', 'cookies', 'achocolatado', 'jujuba', 'jujubas'])
WHERE id='melancia';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') - ARRAY['bala', 'balas', 'bombom', 'bombons', 'chiclete', 'chicletes', 'chicle', 'chicles', 'sorvete', 'sorvetes', 'picole', 'picoles', 'biscoitos', 'bolacha', 'bolachas', 'wafer', 'refrigerante', 'refrigerantes', 'salgadinho', 'cookie', 'cookies', 'achocolatado', 'jujuba', 'jujubas'])
WHERE id='leite';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') - ARRAY['bala', 'balas', 'bombom', 'bombons', 'chiclete', 'chicletes', 'chicle', 'chicles', 'sorvete', 'sorvetes', 'picole', 'picoles', 'biscoitos', 'bolachas', 'wafer', 'chocolate', 'refrigerante', 'refrigerantes', 'salgadinho', 'cookie', 'cookies', 'achocolatado', 'jujuba', 'jujubas'])
WHERE id='aveia';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') - ARRAY['bala', 'balas', 'bombom', 'bombons', 'chiclete', 'chicletes', 'chicle', 'chicles', 'sorvete', 'sorvetes', 'picole', 'picoles', 'biscoitos', 'bolachas', 'wafer', 'chocolate', 'refrigerante', 'refrigerantes', 'salgadinho', 'cookie', 'cookies', 'achocolatado', 'jujuba', 'jujubas'])
WHERE id='manteiga';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') - ARRAY['bala', 'balas', 'bombom', 'bombons', 'chiclete', 'chicletes', 'chicle', 'chicles', 'sorvete', 'sorvetes', 'picole', 'picoles', 'biscoito', 'biscoitos', 'bolacha', 'bolachas', 'wafer', 'chocolate', 'refrigerante', 'refrigerantes', 'salgadinho', 'cookie', 'cookies', 'achocolatado', 'jujuba', 'jujubas'])
WHERE id='cenoura';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') - ARRAY['linguica', 'salsicha', 'presunto', 'mortadela'])
WHERE id='carne-bovina-primeira';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') - ARRAY['linguica', 'salsicha', 'presunto', 'mortadela'])
WHERE id='carne-bovina-acem';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') - ARRAY['linguica', 'salsicha', 'presunto', 'mortadela'])
WHERE id='carne-bovina-musculo';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') - ARRAY['linguica', 'salsicha', 'presunto', 'mortadela'])
WHERE id='pernil-suino';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') - ARRAY['linguica', 'salsicha', 'presunto', 'mortadela'])
WHERE id='lombo-suino';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') - ARRAY['linguica', 'salsicha', 'presunto', 'mortadela'])
WHERE id='frango-peito';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') - ARRAY['linguica', 'salsicha', 'presunto', 'mortadela'])
WHERE id='frango-coxa';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') - ARRAY['linguica', 'salsicha', 'presunto', 'mortadela'])
WHERE id='frango-sobrecoxa';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') - ARRAY['linguica', 'salsicha', 'presunto', 'mortadela'])
WHERE id='frango-inteiro';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') - ARRAY['linguica', 'salsicha', 'presunto', 'mortadela'])
WHERE id='frango-coxa-sobrecoxa';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') - ARRAY['instantanea', 'instantaneos', 'instantaneas'])
WHERE id='macarrao';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') - ARRAY['macarrao', 'lamen', 'instantaneo', 'instantanea'])
WHERE id='feijao';
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') - ARRAY['chips','snack','snacks','sabor','fritas'])
WHERE id IN ('cebola','alho');
