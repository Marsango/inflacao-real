WITH original AS (
    SELECT observation.*, coalesce(membership.representative_id, observation.product_id) AS resolved_id, product.name, product.package_quantity AS canonical_quantity,
        product.package_unit AS canonical_unit, product.package_count AS canonical_count, listing.shop_id, listing.product_url,
        jsonb_build_array(observation.shop_product_id, seller_id, store_id,
            postal_code, city, state, channel) AS stream
    FROM price_observations observation
    LEFT JOIN product_equivalence membership ON membership.product_id = observation.product_id
    JOIN products product ON product.id = coalesce(membership.representative_id, observation.product_id)
    JOIN shop_products listing ON listing.id = observation.shop_product_id
    WHERE ($1::bigint IS NULL OR coalesce(membership.representative_id, observation.product_id) = $1)
        AND observed_at >= ($2::date::timestamp AT TIME ZONE 'America/Sao_Paulo')
        AND observed_at < (($3::date + 1)::timestamp AT TIME ZONE 'America/Sao_Paulo')
), scoped AS (
 SELECT resolved_id AS product_id, name, canonical_quantity, canonical_unit, canonical_count,
 shop_id, product_url, stream, seller_id, store_id, postal_code, city, state, channel,
 observed_at, scraper_id, observation_id, payment_method, installments, membership_required,
 coupon_required, membership_program, minimum_quantity, price_amount, price_currency, price_basis,
 availability, package_quantity, package_unit, package_count FROM original
), bases AS (
    SELECT product_id, CASE WHEN max(canonical_quantity) IS NOT NULL THEN 'package'
        WHEN count(DISTINCT price_basis) = 1 AND min(price_basis) IN ('kg','l','unit')
        THEN min(price_basis) END AS basis
    FROM scoped GROUP BY product_id
), policies AS (
    SELECT product_id, stream, bool_or(payment_method = 'any' AND installments IS NULL
        AND membership_required = false AND coupon_required = false
        AND membership_program IS NULL AND minimum_quantity = 1) IS TRUE AS known
    FROM scoped GROUP BY product_id, stream
), measurements AS (
    SELECT observation.product_id, observation.name, bases.basis,
        observation.stream, policies.known, observation.shop_id, observed_at, scraper_id, observation_id,
        (observed_at AT TIME ZONE 'America/Sao_Paulo')::date AS day,
        min(price_amount * 100) FILTER (WHERE
            availability = 'in_stock' AND price_currency = 'BRL' AND price_basis = bases.basis
            AND installments IS NULL AND membership_program IS NULL
            AND CASE WHEN policies.known THEN payment_method = 'any'
                AND membership_required = false AND coupon_required = false AND minimum_quantity = 1
                ELSE payment_method IN ('any','unknown')
                AND membership_required IS NOT TRUE AND coupon_required IS NOT TRUE
                AND (minimum_quantity IS NULL OR minimum_quantity = 1) END
            AND (bases.basis <> 'package' OR (observation.package_count = observation.canonical_count
            AND CASE observation.package_unit WHEN 'kg' THEN 'g' WHEN 'l' THEN 'ml'
                ELSE observation.package_unit END =
                CASE observation.canonical_unit WHEN 'kg' THEN 'g' WHEN 'l' THEN 'ml' ELSE observation.canonical_unit END
            AND observation.package_quantity * CASE WHEN observation.package_unit IN ('kg','l') THEN 1000 ELSE 1 END =
                observation.canonical_quantity * CASE WHEN observation.canonical_unit IN ('kg','l') THEN 1000 ELSE 1 END))
        ) AS price
    FROM scoped observation JOIN bases USING (product_id) JOIN policies USING (product_id, stream)
    GROUP BY observation.product_id, observation.name, bases.basis, observation.stream, policies.known, observation.shop_id,
        observed_at, scraper_id, observation_id
), latest AS (
    SELECT DISTINCT ON (product_id, stream, day) * FROM measurements
    ORDER BY product_id, stream, day, observed_at DESC, scraper_id, observation_id
), panel AS (
    SELECT product_id, name, basis, bool_or(NOT known AND price IS NOT NULL) AS unknown_conditions, count(DISTINCT stream) AS streams,
        count(DISTINCT shop_id) AS shops, max(observed_at) AS collected_at
    FROM measurements GROUP BY product_id, name, basis
), days AS (
    SELECT $2::date + generate_series(0, $3::date - $2::date) AS day
), contexts AS (
    SELECT product_id, stream, shop_id, max(product_url) AS product_url,
        concat_ws(' · ', max(seller_id), max(store_id), max(postal_code), max(city), max(state), max(channel)) AS label
    FROM scoped GROUP BY product_id, stream, shop_id
), context_series AS (
    SELECT contexts.product_id, contexts.stream, contexts.shop_id, shops.name AS shop_name,
        contexts.label, contexts.product_url,
        jsonb_agg(jsonb_build_object('date', days.day::text, 'amount', latest.price::text)
            ORDER BY days.day) AS points
    FROM contexts CROSS JOIN days JOIN shops ON shops.id = contexts.shop_id
    LEFT JOIN latest ON latest.product_id = contexts.product_id
        AND latest.stream = contexts.stream AND latest.day = days.day
    GROUP BY contexts.product_id, contexts.stream, contexts.shop_id, shops.name, contexts.label, contexts.product_url
)
SELECT panel.product_id::text, panel.name, panel.basis, panel.unknown_conditions, panel.collected_at, panel.streams, panel.shops,
    (SELECT count(DISTINCT shop_id) FROM measurements),
    jsonb_agg(jsonb_build_object('id', context_series.stream::text, 'shopId', shop_id,
        'shopName', shop_name, 'label', label, 'productUrl', product_url, 'points', points)
        ORDER BY shop_id, context_series.stream)
FROM panel JOIN context_series USING (product_id)
GROUP BY panel.product_id, panel.name, panel.basis, panel.unknown_conditions, panel.collected_at, panel.streams, panel.shops
ORDER BY panel.product_id;
