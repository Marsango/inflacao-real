-- +goose Up
CREATE TABLE ingestion_payloads (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    scraper_id varchar(64) NOT NULL,
    submission_id uuid NOT NULL,
    shop_id varchar(64) NOT NULL,
    schema_version varchar(16) NOT NULL CHECK (schema_version = '1.0'),
    created_at timestamptz NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now(),
    payload jsonb,
    status varchar(16) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processed', 'failed')),
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    processed_at timestamptz,
    failed_at timestamptz,
    last_error text,
    payload_removed_at timestamptz,

    CONSTRAINT ingestion_payloads_submission_unique UNIQUE (scraper_id, submission_id),
    CONSTRAINT ingestion_payloads_source_unique UNIQUE (id, scraper_id, shop_id),
    CONSTRAINT ingestion_payloads_labels CHECK (
        btrim(scraper_id) <> '' AND btrim(shop_id) <> ''
    ),
    CONSTRAINT ingestion_payloads_object CHECK (jsonb_typeof(payload) = 'object'),
    CONSTRAINT ingestion_payloads_terminal_state CHECK (
        (status = 'pending' AND processed_at IS NULL AND failed_at IS NULL)
        OR (status = 'processed' AND processed_at IS NOT NULL AND failed_at IS NULL)
        OR (status = 'failed' AND processed_at IS NULL AND failed_at IS NOT NULL)
    ),
    CONSTRAINT ingestion_payloads_retention CHECK (
        (payload IS NOT NULL AND payload_removed_at IS NULL)
        OR (payload IS NULL AND payload_removed_at IS NOT NULL AND status = 'processed')
    )
);

CREATE INDEX ingestion_payloads_pending_idx
    ON ingestion_payloads (received_at, id)
    INCLUDE (next_attempt_at)
    WHERE status = 'pending';

CREATE INDEX ingestion_payloads_cleanup_idx
    ON ingestion_payloads (processed_at, id)
    WHERE status = 'processed' AND payload IS NOT NULL;

-- +goose Down
DROP TABLE ingestion_payloads;
