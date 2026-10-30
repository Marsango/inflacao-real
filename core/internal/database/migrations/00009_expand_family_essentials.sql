-- +goose Up
-- Add ten household essentials and two raw chicken groups.
INSERT INTO monitoring_targets (id, name, criteria) VALUES
('fralda-infantil', 'Fraldas infantis descartáveis', '{"search_queries":["fralda infantil","fralda descartavel"],"required_terms":[["fralda","fraldas"]],"excluded_terms":["adulto","adulta","adultos","adultas","geriatrico","geriatrica","geriatricos","geriatricas","incontinencia","roupa intima","pet","pets","cachorro","cachorros","gato","gatos","cao","caes","veterinario","antipulgas","pano","tecido","ecologica","ecologicas","reutilizavel","reutilizaveis","piscina","natacao","pomada","creme","lenco","lencos","trocador","porta fralda","kit"]}'),
('fralda-adulta', 'Fraldas adultas descartáveis', '{"search_queries":["fralda adulta","fralda geriatrica"],"required_terms":[["fralda","fraldas"],["adulto","adulta","adultos","adultas","geriatrico","geriatrica","geriatricos","geriatricas","incontinencia","roupa intima"]],"excluded_terms":["pet","pets","cachorro","cachorros","gato","gatos","cao","caes","veterinario","antipulgas","infantil","infantis","bebe","bebes","pano","tecido","reutilizavel","reutilizaveis","pomada","creme","lenco","lencos","kit"]}'),
('escova-dental', 'Escova dental manual', '{"search_queries":["escova dental","escova de dente"],"required_terms":[["escova dental","escovas dentais","escova de dente","escova de dentes"]],"excluded_terms":["pet","pets","cachorro","cachorros","gato","gatos","cao","caes","veterinario","antipulgas","eletrica","eletricas","pilha","pilhas","refil","interdental","creme dental","pasta dental","pasta de dente","kit"]}'),
('shampoo', 'Shampoo de uso humano', '{"search_queries":["shampoo","xampu"],"required_terms":[["shampoo","xampu"]],"excluded_terms":["pet","pets","cachorro","cachorros","gato","gatos","cao","caes","veterinario","antipulgas","condicionador","kit","automotivo","carro","carros","lava autos"]}'),
('sabao-roupas-barra', 'Sabão em barra para roupas', '{"search_queries":["sabao em barra","sabao barra"],"required_terms":[["sabao"],["barra","barras"]],"excluded_terms":["sabonete","po","liquido","liquida","pasta","maos","corporal","automotivo","louca","loucas"]}'),
('saco-lixo', 'Sacos para lixo', '{"search_queries":["saco lixo","sacos para lixo"],"required_terms":[["saco","sacos"],["lixo"]],"excluded_terms":["lixeira","suporte","dispensador","dispenser","porta saco","kit"]}'),
('esponja-louca', 'Esponja para lavar louça', '{"search_queries":["esponja louca","esponja multiuso"],"required_terms":[["esponja","esponjas"],["louca","loucas","multiuso","dupla face","nao risca"]],"excluded_terms":["aco","banho","corporal","maquiagem","automotiva","automotivo","magica","melamina","sabonete","kit"]}'),
('desodorante', 'Desodorante corporal', '{"search_queries":["desodorante"],"required_terms":[["desodorante"]],"excluded_terms":["pet","pets","cachorro","cachorros","gato","gatos","cao","caes","veterinario","antipulgas","ambiente","ambientes","sanitario","sanitaria","vaso","pes","pedico","calcado","calcados","tenis","intimo","intima","kit"]}'),
('fio-dental', 'Fio ou fita dental', '{"search_queries":["fio dental","fita dental"],"required_terms":[["fio dental","fita dental"]],"excluded_terms":["passador","suporte","porta fio","haste","hastes","kit","escova","creme dental"]}'),
('frango-inteiro', 'Frango inteiro cru', '{"search_queries":["frango inteiro"],"required_terms":[["frango"],["inteiro"]],"excluded_terms":["empanado","empanada","temperado","temperada","cozido","cozida","assado","assada","pronto","pronta","recheado","recheada","defumado","defumada","racao","petisco","sopa","caldo","lasanha","peito","coxa","sobrecoxa","asa","file","files","kit"]}'),
('frango-coxa-sobrecoxa', 'Coxa e sobrecoxa de frango juntas', '{"search_queries":["coxa sobrecoxa frango","coxa com sobrecoxa"],"required_terms":[["frango"],["coxa"],["sobrecoxa"]],"excluded_terms":["empanado","empanada","temperado","temperada","cozido","cozida","assado","assada","pronto","pronta","recheado","recheada","defumado","defumada","racao","petisco","sopa","caldo","lasanha","peito","asa","kit"]}'),
('desinfetante', 'Desinfetante doméstico líquido', '{"search_queries":["desinfetante"],"required_terms":[["desinfetante"]],"excluded_terms":["aerossol","aerosol","spray","gel","pastilha","pastilhas","lenco","lencos","hospitalar","veterinario","piscina","roupa","roupas","maos","kit"]}');

-- Keep cosmetic ingredients and cleaning fragrances out of food targets.
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') || '["shampoo", "xampu", "desodorante"]'::jsonb)
WHERE id IN ('maca', 'laranja', 'banana', 'mamao', 'limao', 'melancia', 'manteiga', 'oleo-soja', 'leite', 'ovos', 'cenoura', 'aveia');
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') || '["sabao", "desinfetante"]'::jsonb)
WHERE id IN ('banana', 'leite', 'ovos', 'cenoura', 'aveia');

-- +goose Down
DELETE FROM product_monitoring_targets WHERE target_id IN ('fralda-infantil', 'fralda-adulta', 'escova-dental', 'shampoo', 'sabao-roupas-barra', 'saco-lixo', 'esponja-louca', 'desodorante', 'fio-dental', 'frango-inteiro', 'frango-coxa-sobrecoxa', 'desinfetante');
DELETE FROM monitoring_targets WHERE id IN ('fralda-infantil', 'fralda-adulta', 'escova-dental', 'shampoo', 'sabao-roupas-barra', 'saco-lixo', 'esponja-louca', 'desodorante', 'fio-dental', 'frango-inteiro', 'frango-coxa-sobrecoxa', 'desinfetante');
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') - ARRAY['shampoo', 'xampu', 'desodorante'])
WHERE id IN ('maca', 'laranja', 'banana', 'mamao', 'limao', 'melancia', 'manteiga', 'oleo-soja', 'leite', 'ovos', 'cenoura', 'aveia');
UPDATE monitoring_targets SET criteria=jsonb_set(criteria, '{excluded_terms}',
    (criteria->'excluded_terms') - ARRAY['sabao', 'desinfetante'])
WHERE id IN ('banana', 'leite', 'ovos', 'cenoura', 'aveia');
