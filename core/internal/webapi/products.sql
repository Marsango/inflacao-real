WITH eligible AS (
    SELECT coalesce(membership.representative_id, observation.product_id) AS product_id
    FROM price_observations observation
    LEFT JOIN product_equivalence membership ON membership.product_id = observation.product_id
    WHERE availability = 'in_stock' AND price_amount IS NOT NULL
    GROUP BY coalesce(membership.representative_id, observation.product_id),
        shop_product_id, seller_id, store_id, postal_code, city, state, channel
    HAVING count(DISTINCT (observed_at AT TIME ZONE 'America/Sao_Paulo')::date) >= 2
), matching AS (
    SELECT coalesce(membership.representative_id, member.id) AS product_id
    FROM products member
    LEFT JOIN product_equivalence membership ON membership.product_id = member.id
    WHERE $3::text = '' OR member.search_document @@ to_tsquery('simple',
        nullif(array_to_string(tsvector_to_array(
            to_tsvector('simple', product_search_text($3))), ':* & '), '') || ':*')
)
SELECT id::text, name || coalesce(' · ' || brand, '')
FROM products
WHERE id > $1
    AND id IN (SELECT product_id FROM eligible)
    AND id IN (SELECT product_id FROM matching)
    AND (cardinality($4::text[]) = 0 OR id IN (
        SELECT equivalent_product_id(product_id) FROM product_monitoring_targets
        WHERE target_id = ANY($4::text[])))
ORDER BY id
LIMIT $2
