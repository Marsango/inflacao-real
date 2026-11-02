daily_source AS (
    SELECT observation.product_id, observation.scraper_id, observation.observation_id,
        coalesce(observation.last_seen_at, observation.observed_at) AS observed_at,
        observation.package_quantity, observation.price_basis, listing.shop_id, source_days.day,
        jsonb_build_array(shop_product_id, seller_id, store_id, postal_code, city, state, channel) AS stream,
        jsonb_build_array(package_quantity * CASE WHEN package_unit IN ('kg','l') THEN 1000 ELSE 1 END,
            CASE package_unit WHEN 'kg' THEN 'g' WHEN 'l' THEN 'ml' ELSE package_unit END, package_count) AS package_shape,
        (payment_method='any' AND installments IS NULL AND membership_required=false
            AND coupon_required=false AND membership_program IS NULL AND minimum_quantity=1) AS strict_conditions,
        (payment_method IN ('any','unknown') AND installments IS NULL AND membership_required IS NOT TRUE
            AND coupon_required IS NOT TRUE AND membership_program IS NULL
            AND (minimum_quantity IS NULL OR minimum_quantity=1)) AS relaxed_conditions,
        CASE WHEN availability='in_stock' AND price_currency='BRL' THEN price_amount*100 END AS amount
    FROM source_days JOIN price_observations observation
        ON observation.observed_at >= (source_days.day::timestamp AT TIME ZONE 'America/Sao_Paulo')
        AND observation.observed_at < ((source_days.day+1)::timestamp AT TIME ZONE 'America/Sao_Paulo')
    JOIN shop_products listing ON listing.id=observation.shop_product_id
    WHERE observation.product_id IS NOT NULL
), daily_flags AS (
    SELECT product_id, stream, day,
        coalesce(array_agg(DISTINCT price_basis::text) FILTER (WHERE price_basis IS NOT NULL), ARRAY[]::text[]) AS price_bases,
        bool_or(strict_conditions) IS TRUE AS known_conditions,
        coalesce(jsonb_agg(DISTINCT package_shape) FILTER (WHERE relaxed_conditions AND amount IS NOT NULL
            AND price_basis='package' AND package_quantity IS NOT NULL),'[]'::jsonb) AS eligible_packages,
        bool_or(relaxed_conditions AND amount IS NOT NULL AND price_basis='kg') IS TRUE AS eligible_kg,
        bool_or(relaxed_conditions AND amount IS NOT NULL AND price_basis='l') IS TRUE AS eligible_l,
        bool_or(relaxed_conditions AND amount IS NOT NULL AND price_basis='unit') IS TRUE AS eligible_unit
    FROM daily_source GROUP BY product_id,stream,day
), daily_latest AS (
    SELECT DISTINCT ON(product_id,stream,day) product_id,stream,day,observed_at,scraper_id,observation_id
    FROM daily_source ORDER BY product_id,stream,day,observed_at DESC,scraper_id,observation_id
), daily_prices AS (
    SELECT observation.product_id,observation.stream,observation.day,observation.shop_id,
        observation.observed_at,observation.scraper_id,observation.observation_id,observation.package_shape,
        min(amount) FILTER (WHERE strict_conditions AND price_basis='package') AS known_package,
        min(amount) FILTER (WHERE relaxed_conditions AND price_basis='package') AS relaxed_package,
        min(amount) FILTER (WHERE strict_conditions AND price_basis='kg') AS known_kg,
        min(amount) FILTER (WHERE relaxed_conditions AND price_basis='kg') AS relaxed_kg,
        min(amount) FILTER (WHERE strict_conditions AND price_basis='l') AS known_l,
        min(amount) FILTER (WHERE relaxed_conditions AND price_basis='l') AS relaxed_l,
        min(amount) FILTER (WHERE strict_conditions AND price_basis='unit') AS known_unit,
        min(amount) FILTER (WHERE relaxed_conditions AND price_basis='unit') AS relaxed_unit
    FROM daily_source observation JOIN daily_latest USING(product_id,stream,day,observed_at,scraper_id,observation_id)
    GROUP BY observation.product_id,observation.stream,observation.day,observation.shop_id,
        observation.observed_at,observation.scraper_id,observation.observation_id,observation.package_shape
), daily_rows AS (
    SELECT prices.product_id,stream,day,shop_id,observed_at,scraper_id,observation_id,package_shape,
        flags.price_bases,flags.known_conditions,flags.eligible_packages,flags.eligible_kg,flags.eligible_l,flags.eligible_unit,
        known_package,relaxed_package,known_kg,relaxed_kg,known_l,relaxed_l,known_unit,relaxed_unit
    FROM daily_prices prices JOIN daily_flags flags USING(product_id,stream,day)
)
