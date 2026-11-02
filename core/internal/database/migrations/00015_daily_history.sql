-- +goose Up
-- Dirty days are advanced in the same transaction as normalized observations.
CREATE TABLE history_daily_state (
    id boolean PRIMARY KEY DEFAULT true CHECK (id),
    initialized boolean NOT NULL DEFAULT false,
    methodology text NOT NULL DEFAULT ''
);
INSERT INTO history_daily_state DEFAULT VALUES;
CREATE TABLE history_daily_days (
    day date PRIMARY KEY,
    generation bigint NOT NULL DEFAULT 1,
    summarized_generation bigint NOT NULL DEFAULT 0,
    refreshed_at timestamptz
);
CREATE TABLE history_daily (
    product_id bigint NOT NULL REFERENCES products(id),
    stream jsonb NOT NULL,
    day date NOT NULL,
    shop_id varchar(64) NOT NULL REFERENCES shops(id),
    observed_at timestamptz NOT NULL,
    scraper_id varchar(64) NOT NULL,
    observation_id uuid NOT NULL,
    package_shape jsonb NOT NULL,
    price_bases text[] NOT NULL,
    known_conditions boolean NOT NULL,
    eligible_packages jsonb NOT NULL,
    eligible_kg boolean NOT NULL,
    eligible_l boolean NOT NULL,
    eligible_unit boolean NOT NULL,
    known_package numeric,
    relaxed_package numeric,
    known_kg numeric,
    relaxed_kg numeric,
    known_l numeric,
    relaxed_l numeric,
    known_unit numeric,
    relaxed_unit numeric,
    PRIMARY KEY (day, product_id, stream)
);
CREATE INDEX price_observations_day_idx ON price_observations(observed_at);

-- +goose Down
DROP INDEX price_observations_day_idx;
DROP TABLE history_daily, history_daily_days, history_daily_state;
