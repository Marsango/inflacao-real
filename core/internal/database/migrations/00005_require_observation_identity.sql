-- +goose Up
-- Every historical offer must identify its scraper's source measurement.
-- IDs are supplied by the scraper, with no database-generated default.
ALTER TABLE price_observations
    ALTER COLUMN scraper_id SET NOT NULL,
    ALTER COLUMN observation_id SET NOT NULL;

-- +goose Down
ALTER TABLE price_observations
    ALTER COLUMN scraper_id DROP NOT NULL,
    ALTER COLUMN observation_id DROP NOT NULL;
