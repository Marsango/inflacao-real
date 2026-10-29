-- +goose Up
-- History references the payload ID directly; the former composite key is unused.
ALTER TABLE ingestion_payloads DROP CONSTRAINT ingestion_payloads_source_unique;

CREATE TABLE shops (
    id varchar(64) PRIMARY KEY CHECK (btrim(id) <> ''),
    name varchar(128) NOT NULL CHECK (btrim(name) <> ''),
    website_url text NOT NULL CHECK (btrim(website_url) <> '')
);

CREATE TABLE products (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name varchar(512) NOT NULL CHECK (btrim(name) <> ''),
    brand varchar(512),
    gtin varchar(14) CHECK (gtin ~ '^([0-9]{8}|[0-9]{12}|[0-9]{13}|[0-9]{14})$'),
    package_quantity numeric CHECK (
        package_quantity > 0 AND package_quantity <= 999999999.999999
        AND scale(package_quantity) <= 6
    ),
    package_unit varchar(8) CHECK (package_unit IN ('g', 'kg', 'ml', 'l', 'unit')),
    package_count integer CHECK (package_count BETWEEN 1 AND 100000),

    CONSTRAINT products_package_complete
        CHECK (num_nonnulls(package_quantity, package_unit, package_count) IN (0, 3))
);

CREATE TABLE shop_products (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    shop_id varchar(64) NOT NULL REFERENCES shops (id),
    product_id bigint REFERENCES products (id),
    source_product_id varchar(256) NOT NULL CHECK (btrim(source_product_id) <> ''),
    name varchar(512) NOT NULL CHECK (btrim(name) <> ''),
    brand varchar(512),
    gtin varchar(14) CHECK (gtin ~ '^([0-9]{8}|[0-9]{12}|[0-9]{13}|[0-9]{14})$'),
    product_url text NOT NULL CHECK (btrim(product_url) <> ''),

    CONSTRAINT shop_products_source_unique UNIQUE (shop_id, source_product_id)
);

CREATE INDEX shop_products_product_idx ON shop_products (product_id)
    WHERE product_id IS NOT NULL;

CREATE TABLE price_observations (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    payload_id bigint NOT NULL REFERENCES ingestion_payloads (id),
    shop_product_id bigint NOT NULL REFERENCES shop_products (id),
    observed_at timestamptz NOT NULL,
    parsed_at timestamptz NOT NULL DEFAULT now(),
    source_url text NOT NULL,
    package_quantity numeric CHECK (
        package_quantity > 0 AND package_quantity <= 999999999.999999
        AND scale(package_quantity) <= 6
    ),
    package_unit varchar(8) CHECK (package_unit IN ('g', 'kg', 'ml', 'l', 'unit')),
    package_count integer CHECK (package_count BETWEEN 1 AND 100000),
    seller_id varchar(256),
    postal_code varchar(8) CHECK (postal_code ~ '^[0-9]{8}$'),
    city varchar(512),
    state varchar(2) CHECK (state IN (
        'AC', 'AL', 'AP', 'AM', 'BA', 'CE', 'DF', 'ES', 'GO', 'MA', 'MT', 'MS', 'MG',
        'PA', 'PB', 'PR', 'PE', 'PI', 'RJ', 'RN', 'RS', 'RO', 'RR', 'SC', 'SP', 'SE', 'TO'
    )),
    channel varchar(16) NOT NULL CHECK (channel IN ('delivery', 'pickup', 'online', 'unknown')),
    availability varchar(16) NOT NULL CHECK (availability IN ('in_stock', 'out_of_stock', 'unknown')),
    offer_count smallint NOT NULL CHECK (offer_count BETWEEN 0 AND 20),
    offer_index smallint NOT NULL,
    price_amount numeric CHECK (
        price_amount >= 0 AND price_amount <= 999999999.99 AND scale(price_amount) <= 2
    ),
    price_currency varchar(3) CHECK (price_currency = 'BRL'),
    price_basis varchar(16) CHECK (price_basis IN ('package', 'kg', 'l', 'unit')),
    reference_amount numeric CHECK (
        reference_amount >= price_amount AND reference_amount <= 999999999.99
        AND scale(reference_amount) <= 2
    ),
    payment_method varchar(16) CHECK (payment_method IN (
        'any', 'pix', 'cash', 'debit_card', 'credit_card', 'boleto', 'other', 'unknown'
    )),
    installments integer CHECK (installments BETWEEN 1 AND 120),
    membership_required boolean,
    membership_program varchar(512),
    minimum_quantity numeric CHECK (
        minimum_quantity > 0 AND minimum_quantity <= 999999999.999999
        AND scale(minimum_quantity) <= 6
    ),
    condition_description varchar(2048),

    CONSTRAINT price_observations_package_complete
        CHECK (num_nonnulls(package_quantity, package_unit, package_count) IN (0, 3)),
    CONSTRAINT price_observations_offer_shape CHECK (
        (offer_count = 0 AND offer_index = 0 AND num_nonnulls(
            price_amount, price_currency, price_basis, reference_amount, payment_method,
            installments, membership_required, membership_program,
            minimum_quantity, condition_description
        ) = 0)
        OR (offer_count > 0 AND offer_index >= 0 AND offer_index < offer_count
            AND price_amount IS NOT NULL AND price_currency IS NOT NULL
            AND price_basis IS NOT NULL AND payment_method IS NOT NULL)
    ),
    CONSTRAINT price_observations_installment_method
        CHECK (installments IS NULL OR payment_method = 'credit_card'),
    CONSTRAINT price_observations_membership_consistent
        CHECK (membership_required IS DISTINCT FROM false OR membership_program IS NULL)
);

CREATE INDEX price_observations_history_idx
    ON price_observations (shop_product_id, observed_at DESC);

CREATE INDEX price_observations_payload_idx ON price_observations (payload_id);

-- +goose Down
DROP TABLE price_observations;
DROP TABLE shop_products;
DROP TABLE products;
DROP TABLE shops;

ALTER TABLE ingestion_payloads ADD CONSTRAINT ingestion_payloads_source_unique
    UNIQUE (id, scraper_id, shop_id);
