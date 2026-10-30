-- +goose Up
-- Normalize names and queries identically, without stemming brand names or
-- removing words. Separate quantities so both "500g" and "500 g" are searchable.
-- +goose StatementBegin
CREATE FUNCTION product_search_text(value text) RETURNS text
LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE AS $$
    SELECT regexp_replace(regexp_replace(regexp_replace(
        translate(lower(normalize(value, NFC)),
            'áàâãäéèêëíìîïóòôõöúùûüç', 'aaaaaeeeeiiiiooooouuuuc'),
        '([0-9])([a-z])', '\1 \2', 'g'),
        '([a-z])([0-9])', '\1 \2', 'g'), '[^a-z0-9]+', ' ', 'g')
$$;
-- +goose StatementEnd

ALTER TABLE products ADD COLUMN search_document tsvector GENERATED ALWAYS AS (
    to_tsvector('simple', product_search_text(name || ' ' || coalesce(brand, '')))
) STORED;
CREATE INDEX products_search_idx ON products USING gin (search_document);

-- +goose Down
ALTER TABLE products DROP COLUMN search_document;
DROP FUNCTION product_search_text(text);
