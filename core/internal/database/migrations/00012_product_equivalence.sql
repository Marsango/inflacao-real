-- +goose Up
CREATE TABLE catalog_decisions (
    id boolean PRIMARY KEY DEFAULT true CHECK (id),
    revision bigint NOT NULL DEFAULT 0,
    generic_enabled boolean NOT NULL DEFAULT false
);
INSERT INTO catalog_decisions DEFAULT VALUES;
CREATE TABLE product_equivalence (
    product_id bigint PRIMARY KEY REFERENCES products(id),
    representative_id bigint NOT NULL REFERENCES products(id)
);
CREATE INDEX product_equivalence_representative ON product_equivalence(representative_id);
CREATE TABLE product_exclusions (
    first_id bigint NOT NULL REFERENCES products(id),
    second_id bigint NOT NULL REFERENCES products(id),
    PRIMARY KEY(first_id, second_id), CHECK(first_id < second_id)
);
CREATE TABLE equivalence_audit (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    actor text NOT NULL, action text NOT NULL, reason text NOT NULL,
    rule_version text, revision bigint NOT NULL,
    before_membership jsonb NOT NULL, after_membership jsonb NOT NULL, decision jsonb NOT NULL
);
CREATE TABLE admin_sessions (
    token_hash text PRIMARY KEY, csrf_token text NOT NULL,
    expires_at timestamptz NOT NULL
);
CREATE INDEX admin_sessions_expiry ON admin_sessions(expires_at);
CREATE TABLE admin_login_throttle (
    identity text PRIMARY KEY, attempts integer NOT NULL, reset_at timestamptz NOT NULL
);
-- +goose StatementBegin
CREATE FUNCTION equivalent_product_id(identity bigint) RETURNS bigint
LANGUAGE sql STABLE AS $$
 SELECT COALESCE((SELECT representative_id FROM product_equivalence WHERE product_id=identity), identity)
$$;
-- +goose StatementEnd
-- +goose Down
DROP FUNCTION equivalent_product_id(bigint);
DROP TABLE admin_login_throttle, admin_sessions, equivalence_audit, product_exclusions, product_equivalence, catalog_decisions;
