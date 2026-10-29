-- +goose Up
-- Matching compares all GTIN forms in their zero-padded 14-digit representation.
CREATE INDEX products_gtin_match_idx ON products (lpad(gtin,14,'0'))
    WHERE gtin IS NOT NULL;

-- A listing can change variant; retain the product selected for each measurement.
-- Legacy rows remain unknown rather than borrowing today's listing assignment.
ALTER TABLE price_observations
    ADD COLUMN product_id bigint REFERENCES products (id),
    ADD COLUMN product_match_method varchar(16)
        CHECK (product_match_method IN ('gtin_v1','attributes_v1','listing_v1','created_v1')),
    ADD CONSTRAINT price_observations_product_match_complete
        CHECK (num_nonnulls(product_id,product_match_method) IN (0,2));

CREATE INDEX price_observations_product_history_idx
    ON price_observations (product_id,observed_at DESC) WHERE product_id IS NOT NULL;

-- +goose Down
DROP INDEX price_observations_product_history_idx;
ALTER TABLE price_observations
    DROP CONSTRAINT price_observations_product_match_complete,
    DROP COLUMN product_match_method,
    DROP COLUMN product_id;
DROP INDEX products_gtin_match_idx;
