-- +goose Up
-- Reuse discovery labels for curated filters, including products collected before
-- labeling was introduced. These helpers only exist for this migration.
-- +goose StatementBegin
CREATE FUNCTION backfill_label_text(value text) RETURNS text
LANGUAGE sql IMMUTABLE STRICT AS $$
    SELECT trim(regexp_replace(regexp_replace(lower(normalize(value, NFD)),
        U&'[\0300-\036f]', '', 'g'), '[^a-z0-9]+', ' ', 'g'))
$$;
-- +goose StatementEnd

-- Same whole-term AND-of-ORs and exclusions as monitoring.Target.Matches.
-- +goose StatementBegin
CREATE FUNCTION backfill_label_matches(name text, criteria jsonb) RETURNS boolean
LANGUAGE sql IMMUTABLE STRICT AS $$
    SELECT jsonb_array_length(criteria->'required_terms') > 0
    AND NOT EXISTS (
        SELECT 1 FROM jsonb_array_elements(criteria->'required_terms') required(terms)
        WHERE NOT EXISTS (
            SELECT 1 FROM jsonb_array_elements_text(required.terms) term(value)
            WHERE backfill_label_text(term.value) <> '' AND strpos(' ' || backfill_label_text(name) || ' ',
                ' ' || backfill_label_text(term.value) || ' ') > 0))
    AND NOT EXISTS (
        SELECT 1 FROM jsonb_array_elements_text(criteria->'excluded_terms') term(value)
        WHERE backfill_label_text(term.value) <> '' AND strpos(' ' || backfill_label_text(name) || ' ',
            ' ' || backfill_label_text(term.value) || ' ') > 0)
$$;
-- +goose StatementEnd

WITH names AS (
    SELECT id AS product_id, name FROM products
    UNION SELECT product_id, name FROM shop_products WHERE product_id IS NOT NULL
)
INSERT INTO product_monitoring_targets (product_id, target_id)
SELECT DISTINCT names.product_id, target.id FROM names CROSS JOIN monitoring_targets target
WHERE target.enabled AND backfill_label_matches(names.name, target.criteria)
ON CONFLICT DO NOTHING;

DROP FUNCTION backfill_label_matches(text, jsonb);
DROP FUNCTION backfill_label_text(text);
UPDATE catalog_decisions SET revision=revision+1;

-- +goose Down
-- Labels belong to the existing monitoring catalog. Preserve them on rollback,
-- including associations subsequently reinforced by ingestion.
SELECT 1;
