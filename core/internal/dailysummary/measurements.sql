, combined AS (
    SELECT * FROM history_daily stored WHERE day BETWEEN $2::date AND $3::date
        AND NOT EXISTS(SELECT 1 FROM source_days WHERE source_days.day=stored.day)
    UNION ALL SELECT * FROM daily_rows
), scoped AS (
    SELECT observation.*,coalesce(membership.representative_id,observation.product_id) AS resolved_id,
        product.package_quantity AS canonical_quantity,
        jsonb_build_array(product.package_quantity * CASE WHEN product.package_unit IN ('kg','l') THEN 1000 ELSE 1 END,
            CASE product.package_unit WHEN 'kg' THEN 'g' WHEN 'l' THEN 'ml' ELSE product.package_unit END,
            product.package_count) AS canonical_shape
    FROM combined observation
    LEFT JOIN product_equivalence membership ON membership.product_id=observation.product_id
    JOIN products product ON product.id=coalesce(membership.representative_id,observation.product_id)
    WHERE ($1::bigint IS NULL OR product.id=$1)
        AND (cardinality($4::text[])=0 OR product.id IN (
            SELECT equivalent_product_id(product_id) FROM product_monitoring_targets WHERE target_id=ANY($4::text[])))
), bases AS (
    SELECT resolved_id, CASE WHEN max(canonical_quantity) IS NOT NULL THEN 'package'
        WHEN count(DISTINCT price_basis)=1 AND min(price_basis) IN ('kg','l','unit') THEN min(price_basis) END AS basis
    FROM scoped LEFT JOIN LATERAL unnest(price_bases) price_basis ON true GROUP BY resolved_id
), policies AS (
    SELECT resolved_id,stream,bool_or(known_conditions) AS known FROM scoped GROUP BY resolved_id,stream
), measurements AS (
    SELECT resolved_id AS product_id,observation.stream,day,shop_id,observed_at,scraper_id,observation_id,basis,known,
        CASE basis
            WHEN 'package' THEN CASE WHEN package_shape=canonical_shape
                THEN CASE WHEN known THEN known_package ELSE relaxed_package END END
            WHEN 'kg' THEN CASE WHEN known THEN known_kg ELSE relaxed_kg END
            WHEN 'l' THEN CASE WHEN known THEN known_l ELSE relaxed_l END
            WHEN 'unit' THEN CASE WHEN known THEN known_unit ELSE relaxed_unit END END AS price,
        NOT known AND CASE basis
            WHEN 'package' THEN eligible_packages @> jsonb_build_array(canonical_shape)
            WHEN 'kg' THEN eligible_kg WHEN 'l' THEN eligible_l WHEN 'unit' THEN eligible_unit
            ELSE false END AS unknown_conditions
    FROM scoped observation JOIN bases USING(resolved_id) JOIN policies USING(resolved_id,stream)
), latest AS (
    SELECT DISTINCT ON(product_id,stream,day) * FROM measurements
    ORDER BY product_id,stream,day,observed_at DESC,scraper_id,observation_id
), panel AS (
    SELECT product_id,basis,bool_or(unknown_conditions) AS unknown_conditions,max(observed_at) AS collected_at,
        count(DISTINCT stream) AS streams,count(DISTINCT shop_id) AS shops
    FROM measurements GROUP BY product_id,basis
)
