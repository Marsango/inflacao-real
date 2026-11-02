WITH incoming AS (
    SELECT * FROM jsonb_populate_recordset(NULL::price_observations, $3::jsonb)
), latest AS (
    SELECT stored.scraper_id, stored.observation_id
    FROM price_observations stored CROSS JOIN (SELECT * FROM incoming LIMIT 1) candidate
    WHERE stored.shop_product_id = $1 AND stored.scraper_id = $2
        AND stored.observed_at >= (
            ($4::timestamptz AT TIME ZONE 'America/Sao_Paulo')::date::timestamp
            AT TIME ZONE 'America/Sao_Paulo')
        AND stored.observed_at < (
            (($4::timestamptz AT TIME ZONE 'America/Sao_Paulo')::date + 1)::timestamp
            AT TIME ZONE 'America/Sao_Paulo')
        AND ROW(stored.seller_id, stored.store_id, stored.postal_code,
                stored.city, stored.state, stored.channel)
            IS NOT DISTINCT FROM
            ROW(candidate.seller_id, candidate.store_id, candidate.postal_code,
                candidate.city, candidate.state, candidate.channel)
    ORDER BY coalesce(stored.last_seen_at, stored.observed_at) DESC,
        stored.observed_at DESC, stored.observation_id, stored.offer_index
    LIMIT 1
), ignored AS (
    SELECT ARRAY['id', 'payload_id', 'shop_product_id', 'scraper_id', 'observation_id',
        'observed_at', 'parsed_at', 'last_seen_at', 'source_url', 'collection_method',
        'product_match_method', 'offer_index']::text[] AS columns
), candidate_states AS (
    SELECT to_jsonb(candidate) - ignored.columns AS state FROM incoming candidate CROSS JOIN ignored
), stored_states AS (
    SELECT to_jsonb(stored) - ignored.columns AS state
    FROM price_observations stored JOIN latest USING (scraper_id, observation_id) CROSS JOIN ignored
), unchanged AS (
    SELECT (SELECT jsonb_agg(state ORDER BY state::text) FROM candidate_states) =
        (SELECT jsonb_agg(state ORDER BY state::text) FROM stored_states) AS matches
)
UPDATE price_observations stored
SET last_seen_at = greatest(coalesce(stored.last_seen_at, stored.observed_at), $4::timestamptz)
FROM latest, unchanged
WHERE unchanged.matches AND stored.scraper_id = latest.scraper_id
    AND stored.observation_id = latest.observation_id;
