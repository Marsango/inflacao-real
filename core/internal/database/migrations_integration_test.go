package database

import (
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestMigrationsUpRepeatDownAndUp(t *testing.T) {
	ctx, db := testDatabase(t)
	for range 2 {
		if err := Migrate(ctx, db); err != nil {
			t.Fatal(err)
		}
	}
	provider, err := migrationProvider(db)
	if err != nil {
		t.Fatal(err)
	}
	version, err := provider.GetDBVersion(ctx)
	if err != nil || version != 5 {
		t.Fatalf("expected migration version 5; got %d, error %v", version, err)
	}
	if _, err := provider.DownTo(ctx, 0); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM information_schema.tables
		WHERE table_schema = current_schema() AND table_name IN
		('ingestion_payloads', 'shops', 'products', 'shop_products', 'price_observations', 'observation_identities')`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rollback left application tables: count %d, error %v", count, err)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM information_schema.tables
		WHERE table_schema = current_schema() AND table_name IN
		('ingestion_payloads', 'shops', 'products', 'shop_products', 'price_observations', 'observation_identities')`).Scan(&count); err != nil || count != 6 {
		t.Fatalf("expected six application tables: count %d, error %v", count, err)
	}
	var required bool
	if err := db.QueryRowContext(ctx, `SELECT count(*)=2 AND bool_and(is_nullable='NO' AND column_default IS NULL)
		FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='price_observations'
		AND column_name IN ('scraper_id','observation_id')`).Scan(&required); err != nil || !required {
		t.Fatalf("observation identity must be required without generated defaults: %t, %v", required, err)
	}
}

func TestCatalogHistoryAndPayloadRetention(t *testing.T) {
	ctx, db := testDatabase(t)
	payload, err := os.ReadFile("../../../docs/contracts/ingestion/v1/examples/submission.json")
	if err != nil {
		t.Fatal(err)
	}
	var payloadID int64
	const insertPayload = `INSERT INTO ingestion_payloads (
		scraper_id, submission_id, shop_id, schema_version, created_at, payload
	) VALUES ('collector-sp-01', 'd6d60e4f-7320-4c69-9d17-f0ee7c44b131',
		'mercado-exemplo', '1.0', now(), $1::jsonb) RETURNING id`
	err = db.QueryRowContext(ctx, insertPayload, string(payload)).Scan(&payloadID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO shops (id, name, website_url) VALUES
		('mercado-exemplo', 'Mercado Exemplo', 'https://mercado.example.com'),
		('outro-mercado', 'Outro Mercado', 'https://outro.example.com')`); err != nil {
		t.Fatal(err)
	}
	var listingID int64
	if err := db.QueryRowContext(ctx, `INSERT INTO shop_products
		(shop_id, source_product_id, name, brand, product_url)
		VALUES ('mercado-exemplo', 'sku-000123', 'Arroz branco 5 kg', 'Marca Exemplo',
		'https://mercado.example.com/products/arroz-5kg') RETURNING id`).Scan(&listingID); err != nil {
		t.Fatal(err)
	}
	var matched bool
	if err := db.QueryRowContext(ctx, `SELECT product_id IS NOT NULL FROM shop_products
		WHERE id = $1`, listingID).Scan(&matched); err != nil || matched {
		t.Fatalf("new listings must allow an unmatched product: matched %t, error %v", matched, err)
	}
	const regularID = "33c8b8f4-3bbb-47ed-8814-9fa144f70f86"
	const emptyID = "00000000-0000-4000-8000-000000000001"
	const invalidID = "00000000-0000-4000-8000-000000000002"
	for _, observationID := range []string{regularID, emptyID, invalidID} {
		if _, err := db.ExecContext(ctx, `INSERT INTO observation_identities
			(scraper_id,observation_id,fingerprint,payload_id,run_id,collector_name,collector_version)
			SELECT scraper_id,$2::uuid,decode(repeat('00',32),'hex'),id,
			(payload->>'run_id')::uuid,payload->'collector'->>'name',payload->'collector'->>'version'
			FROM ingestion_payloads WHERE id=$1`, payloadID, observationID); err != nil {
			t.Fatal(err)
		}
	}
	const insertObservation = `INSERT INTO price_observations (
		payload_id, shop_product_id, observed_at, source_url,
		package_quantity, package_unit, package_count, channel,
		availability, offer_count, offer_index, price_amount, price_currency,
		price_basis, payment_method, scraper_id, observation_id
	) VALUES ($1, $2, now(),
		'https://mercado.example.com/api/products/arroz-5kg',
		5, 'kg', 1, 'online', 'in_stock',
		$3, $4, $5, $6, $7, $8, 'collector-sp-01', $9)`
	insert := func(listing int64, count, index int, amount, currency, basis, method any) error {
		observationID := invalidID
		if count == 2 {
			observationID = regularID
		} else if count == 0 {
			observationID = emptyID
		}
		_, err := db.ExecContext(ctx, insertObservation, payloadID, listing,
			count, index, amount, currency, basis, method, observationID)
		return err
	}
	for index, amount := range []string{"24.90", "22.90"} {
		if err := insert(listingID, 2, index, amount, "BRL", "package", "any"); err != nil {
			t.Fatal(err)
		}
	}
	assertCode := func(err error, code string) {
		t.Helper()
		var postgresError *pgconn.PgError
		if !errors.As(err, &postgresError) || postgresError.Code != code {
			t.Fatalf("expected PostgreSQL error %s; got %v", code, err)
		}
	}
	assertCode(insert(listingID, 1, 0, "24.901", "BRL", "package", "any"), "23514")
	assertCode(insert(-1, 1, 0, "24.90", "BRL", "package", "any"), "23503")
	assertCode(insert(listingID, 0, 0, "0.00", "BRL", "package", "any"), "23514")
	_, err = db.ExecContext(ctx, `UPDATE price_observations SET observation_id=NULL WHERE payload_id=$1`, payloadID)
	assertCode(err, "23502")
	_, err = db.ExecContext(ctx, `UPDATE price_observations SET scraper_id=NULL WHERE payload_id=$1`, payloadID)
	assertCode(err, "23502")
	if err := insert(listingID, 0, 0, nil, nil, nil, nil); err != nil {
		t.Fatalf("empty offers must allow a null-price observation: %v", err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO shop_products
		(shop_id, source_product_id, name, product_url)
		SELECT shop_id, source_product_id, name, product_url FROM shop_products WHERE id = $1`, listingID)
	assertCode(err, "23505")
	_, err = db.ExecContext(ctx, `UPDATE shop_products SET product_id = -1 WHERE id = $1`, listingID)
	assertCode(err, "23503")
	var productID int64
	if err := db.QueryRowContext(ctx, `INSERT INTO products
		(name, brand, package_quantity, package_unit, package_count)
		VALUES ('Arroz branco 5 kg', 'Marca Exemplo', 5, 'kg', 1) RETURNING id`).Scan(&productID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE shop_products SET product_id = $1,
		name = 'Novo nome de exibição' WHERE id = $2`, productID, listingID); err != nil {
		t.Fatal(err)
	}
	// The same source SKU in another shop is a separate listing of the same product.
	if _, err := db.ExecContext(ctx, `INSERT INTO shop_products
		(shop_id, product_id, source_product_id, name, product_url)
		VALUES ('outro-mercado', $1, 'sku-000123', 'Arroz branco',
		'https://outro.example.com/products/arroz')`, productID); err != nil {
		t.Fatal(err)
	}
	var listings int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM shop_products
		WHERE product_id = $1`, productID).Scan(&listings); err != nil || listings != 2 {
		t.Fatalf("expected two shops linked to one product: listings %d, error %v", listings, err)
	}
	// Catalog matching and metadata changes must leave the collected measurement intact.
	var unchanged bool
	if err := db.QueryRowContext(ctx, `SELECT price_amount = 24.90
		AND package_quantity = 5 AND package_unit = 'kg' AND package_count = 1
		FROM price_observations WHERE shop_product_id = $1 AND offer_count = 2
		AND offer_index = 0`, listingID).Scan(&unchanged); err != nil || !unchanged {
		t.Fatalf("catalog update changed historical measurement: unchanged %t, error %v", unchanged, err)
	}
	_, err = db.ExecContext(ctx, `DELETE FROM shop_products WHERE id = $1`, listingID)
	assertCode(err, "23503")
	_, err = db.ExecContext(ctx, `DELETE FROM products WHERE id = $1`, productID)
	assertCode(err, "23503")
	_, err = db.ExecContext(ctx, `UPDATE ingestion_payloads SET payload = NULL, payload_removed_at = now() WHERE id = $1`, payloadID)
	assertCode(err, "23514")
	_, err = db.ExecContext(ctx, `UPDATE ingestion_payloads SET status = 'processed',
		processed_at = now(), payload = NULL, payload_removed_at = now() WHERE id = $1`, payloadID)
	if err != nil {
		t.Fatal(err)
	}
	// Submission identity still prevents retries after the original body is cleared.
	_, err = db.ExecContext(ctx, insertPayload, `{}`)
	assertCode(err, "23505")
	_, err = db.ExecContext(ctx, `DELETE FROM ingestion_payloads WHERE id = $1`, payloadID)
	assertCode(err, "23503")
	var rows int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM price_observations WHERE payload_id = $1`, payloadID).Scan(&rows); err != nil || rows != 3 {
		t.Fatalf("retention must preserve parsed history: rows %d, error %v", rows, err)
	}
}
