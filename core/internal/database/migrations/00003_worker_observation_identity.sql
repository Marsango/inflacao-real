-- +goose Up
-- Compare timestamps on the locked listing row so late uploads cannot replace
-- newer metadata, including when two payloads process concurrently.
ALTER TABLE shop_products ADD COLUMN metadata_observed_at timestamptz;
UPDATE shop_products s SET metadata_observed_at=(
    SELECT max(observed_at) FROM price_observations p WHERE p.shop_product_id=s.id
);

-- One identity per source measurement, independent of submission and offer count.
CREATE TABLE observation_identities (
    scraper_id varchar(64) NOT NULL,
    observation_id uuid NOT NULL,
    fingerprint bytea NOT NULL CHECK (octet_length(fingerprint) = 32),
    payload_id bigint NOT NULL REFERENCES ingestion_payloads (id),
    run_id uuid NOT NULL,
    collector_name varchar(64) NOT NULL,
    collector_version varchar(128) NOT NULL,
    PRIMARY KEY (scraper_id, observation_id)
);

-- Nullable identity columns allow pre-worker history to remain intact.
ALTER TABLE price_observations
    ADD COLUMN scraper_id varchar(64),
    ADD COLUMN observation_id uuid,
    ADD COLUMN collection_method varchar(16)
        CHECK (collection_method IN ('api', 'html', 'browser')),
    ADD COLUMN store_id varchar(256),
    ADD COLUMN coupon_required boolean,
    ADD CONSTRAINT price_observations_identity_complete
        CHECK (num_nonnulls(scraper_id, observation_id) IN (0, 2)),
    ADD CONSTRAINT price_observations_identity_fk
        FOREIGN KEY (scraper_id, observation_id)
        REFERENCES observation_identities (scraper_id, observation_id),
    ADD CONSTRAINT price_observations_identity_offer_unique
        UNIQUE (scraper_id, observation_id, offer_index),
    ADD CONSTRAINT price_observations_empty_coupon
        CHECK (offer_count > 0 OR coupon_required IS NULL);

-- +goose Down
ALTER TABLE price_observations
    DROP CONSTRAINT price_observations_empty_coupon,
    DROP CONSTRAINT price_observations_identity_offer_unique,
    DROP CONSTRAINT price_observations_identity_fk,
    DROP CONSTRAINT price_observations_identity_complete,
    DROP COLUMN coupon_required,
    DROP COLUMN store_id,
    DROP COLUMN collection_method,
    DROP COLUMN observation_id,
    DROP COLUMN scraper_id;
DROP TABLE observation_identities;
ALTER TABLE shop_products DROP COLUMN metadata_observed_at;
