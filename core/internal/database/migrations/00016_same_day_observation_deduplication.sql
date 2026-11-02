-- +goose Up
-- Preserve original observation provenance while recording unchanged confirmations.
ALTER TABLE price_observations ADD COLUMN last_seen_at timestamptz;
ALTER TABLE price_observations ADD CONSTRAINT price_observations_last_seen_day CHECK (
    last_seen_at IS NULL OR (
        last_seen_at >= observed_at
        AND (last_seen_at AT TIME ZONE 'America/Sao_Paulo')::date =
            (observed_at AT TIME ZONE 'America/Sao_Paulo')::date
    )
);

-- +goose Down
ALTER TABLE price_observations DROP COLUMN last_seen_at;
