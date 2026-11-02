-- +goose Up
-- Persist complete calculated selections: eligibility depends on the requested
-- interval, so an unconditional daily minimum would change the methodology.
CREATE TABLE history_precomputations (
    identity text PRIMARY KEY,
    request jsonb NOT NULL,
    methodology text NOT NULL,
    revision bigint NOT NULL DEFAULT -1,
    result bytea CHECK (octet_length(result) <= 1048576),
    requested_at timestamptz NOT NULL DEFAULT now(),
    computed_at timestamptz
);
CREATE TABLE history_refresh_requests (
    id boolean PRIMARY KEY DEFAULT true CHECK (id),
    generation bigint NOT NULL DEFAULT 1,
    completed_generation bigint NOT NULL DEFAULT 0,
    requested_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);
INSERT INTO history_refresh_requests DEFAULT VALUES;

-- +goose Down
DROP TABLE history_refresh_requests, history_precomputations;
