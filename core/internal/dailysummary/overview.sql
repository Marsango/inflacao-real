 , bounds AS (
    SELECT min(day) FILTER (WHERE price IS NOT NULL) AS first_day,
        max(day) FILTER (WHERE price IS NOT NULL) AS last_day FROM latest
), panel_points AS (
    SELECT latest.*,first_day,last_day,
        max(price) FILTER (WHERE day=first_day) OVER context_window AS baseline,
        max(price) FILTER (WHERE day=last_day) OVER context_window AS ending
    FROM latest CROSS JOIN bounds
    WINDOW context_window AS (PARTITION BY product_id,stream)
), comparable AS (
    SELECT * FROM panel_points WHERE baseline>0 AND ending IS NOT NULL
), shop_weights AS (
    SELECT comparable.*,
        count(*) FILTER (WHERE day=first_day) OVER (PARTITION BY product_id,shop_id) AS contexts,
        dense_rank() OVER (PARTITION BY product_id ORDER BY shop_id) AS shop_rank
    FROM comparable
), weighted_points AS (
    SELECT shop_weights.*,baseline*contexts*max(shop_rank) OVER (PARTITION BY product_id) AS denominator
    FROM shop_weights
), terms AS (
    SELECT day,denominator,sum(price)*10000 AS numerator,count(price) AS priced_streams
    FROM weighted_points GROUP BY day,denominator
), metadata AS (
    SELECT jsonb_build_object('counts',jsonb_build_object(
        'products',(SELECT count(*) FROM panel),'shops',(SELECT count(DISTINCT shop_id) FROM measurements),
        'streams',(SELECT coalesce(sum(streams),0) FROM panel),
        'includedProducts',(SELECT count(DISTINCT product_id) FROM comparable),
        'includedShops',(SELECT count(DISTINCT shop_id) FROM comparable),
        'includedStreams',(SELECT count(*) FROM comparable WHERE day=first_day)),
        'unknownConditions',(SELECT coalesce(bool_or(unknown_conditions),false) FROM panel),
        'collectedAt',(SELECT max(collected_at) FROM panel),
        'availablePeriod',CASE WHEN first_day IS NOT NULL THEN jsonb_build_object('start',first_day::text,'end',last_day::text) END,
        'locations',(SELECT jsonb_agg(DISTINCT coalesce(nullif(concat_ws('/',stream->>4,stream->>5),''),'localidade não informada')) FROM measurements)) AS data
    FROM bounds
)
SELECT 0 AS kind,NULL::text AS day,NULL::text AS denominator,NULL::text AS numerator,0::bigint AS priced_streams,data
FROM metadata
UNION ALL SELECT 1,day::text,denominator::text,numerator::text,priced_streams,NULL::jsonb FROM terms
ORDER BY kind,day,denominator;
