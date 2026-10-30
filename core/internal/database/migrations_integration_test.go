package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pressly/goose/v3"
)

const applicationTablesSQL = `SELECT count(*) FROM information_schema.tables
		WHERE table_schema = current_schema() AND table_name IN
		('ingestion_payloads', 'shops', 'products', 'shop_products', 'price_observations', 'observation_identities', 'monitoring_targets', 'product_monitoring_targets')`

const requiredIdentitySQL = `SELECT count(*)=2 AND bool_and(is_nullable='NO' AND column_default IS NULL)
		FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='price_observations'
		AND column_name IN ('scraper_id','observation_id')`

const insertHistoryPayloadSQL = `INSERT INTO ingestion_payloads (
		scraper_id, submission_id, shop_id, schema_version, created_at, payload
	) VALUES ('collector-sp-01', 'd6d60e4f-7320-4c69-9d17-f0ee7c44b131',
		'mercado-exemplo', '1.0', now(), $1::jsonb) RETURNING id`

const insertHistoryObservationSQL = `INSERT INTO price_observations (
		payload_id, shop_product_id, observed_at, source_url,
		package_quantity, package_unit, package_count, channel,
		availability, offer_count, offer_index, price_amount, price_currency,
		price_basis, payment_method, scraper_id, observation_id
	) VALUES ($1, $2, now(),
		'https://mercado.example.com/api/products/arroz-5kg',
		5, 'kg', 1, 'online', 'in_stock',
		$3, $4, $5, $6, $7, $8, 'collector-sp-01', $9)`

const registerHistoryShopsSQL = `INSERT INTO shops (id, name, website_url) VALUES
		('mercado-exemplo', 'Mercado Exemplo', 'https://mercado.example.com'),
		('outro-mercado', 'Outro Mercado', 'https://outro.example.com')`

const insertHistoryListingSQL = `INSERT INTO shop_products
		(shop_id, source_product_id, name, brand, product_url)
		VALUES ('mercado-exemplo', 'sku-000123', 'Arroz branco 5 kg', 'Marca Exemplo',
		'https://mercado.example.com/products/arroz-5kg') RETURNING id`

const insertHistoryIdentitySQL = `INSERT INTO observation_identities
			(scraper_id,observation_id,fingerprint,payload_id,run_id,collector_name,collector_version)
			SELECT scraper_id,$2::uuid,decode(repeat('00',32),'hex'),id,
			(payload->>'run_id')::uuid,payload->'collector'->>'name',payload->'collector'->>'version'
			FROM ingestion_payloads WHERE id=$1`

const canonicalHistoryProductSQL = `INSERT INTO products
		(name, brand, package_quantity, package_unit, package_count)
		VALUES ('Arroz branco 5 kg', 'Marca Exemplo', 5, 'kg', 1) RETURNING id`

const retainedMeasurementSQL = `SELECT price_amount = 24.90
		AND package_quantity = 5 AND package_unit = 'kg' AND package_count = 1
		FROM price_observations WHERE shop_product_id = $1 AND offer_count = 2
		AND offer_index = 0`

func TestMigrationsUpRepeatDownAndUp(test *testing.T) {
	requestContext, pool := testDatabase(test)
	requireNoError(test, Migrate(requestContext, pool))
	requireNoError(test, Migrate(requestContext, pool))
	provider, err := migrationProvider(pool)
	requireNoError(test, err)
	assertMigrationVersion(test, requestContext, provider, 10)
	_, err = provider.DownTo(requestContext, 0)
	requireNoError(test, err)
	assertApplicationTables(test, requestContext, pool, 0)
	requireNoError(test, Migrate(requestContext, pool))
	assertApplicationTables(test, requestContext, pool, 8)
	assertWorkerSQL(test, requestContext, pool, requiredIdentitySQL)
}

func assertMigrationVersion(test *testing.T, requestContext context.Context, provider *goose.Provider, expected int64) {
	version, err := provider.GetDBVersion(requestContext)
	if err != nil || version != expected {
		test.Fatalf("expected migration version %d; got %d, error %v", expected, version, err)
	}
}

func assertApplicationTables(test *testing.T, requestContext context.Context, pool *sql.DB, expected int) {
	row := pool.QueryRowContext(requestContext, applicationTablesSQL)
	var count int
	if err := row.Scan(&count); err != nil || count != expected {
		test.Fatalf("expected %d application tables: count %d, error %v", expected, count, err)
	}
}

type historyFixture struct {
	workerFixture
	payloadID, listingID int64
}

func newHistoryFixture(test *testing.T) historyFixture {
	fixture := newWorkerFixture(test)
	payload := readExample(test, "submission")
	payloadID := fixture.queryIdentity(insertHistoryPayloadSQL, string(payload))
	fixture.execute(registerHistoryShopsSQL)
	listingID := fixture.queryIdentity(insertHistoryListingSQL)
	return historyFixture{fixture, payloadID, listingID}
}

func TestCatalogHistoryAndPayloadRetention(test *testing.T) {
	fixture := newHistoryFixture(test)
	fixture.assert(`SELECT product_id IS NULL FROM shop_products WHERE id=$1`, fixture.listingID)
	fixture.seedIdentities()
	fixture.insertRegularOffers()
	fixture.checkMoneyAndIdentityConstraints()
	fixture.checkListingConstraints()
	productID := fixture.matchProducts()
	fixture.assert(retainedMeasurementSQL, fixture.listingID)
	fixture.expectFailure(`DELETE FROM shop_products WHERE id=$1`, "23503", fixture.listingID)
	fixture.expectFailure(`DELETE FROM products WHERE id=$1`, "23503", productID)
	fixture.checkRetention()
}

const regularObservationID = "33c8b8f4-3bbb-47ed-8814-9fa144f70f86"
const emptyObservationID = "00000000-0000-4000-8000-000000000001"
const invalidObservationID = "00000000-0000-4000-8000-000000000002"

func (fixture historyFixture) seedIdentities() {
	for _, identity := range []string{regularObservationID, emptyObservationID, invalidObservationID} {
		fixture.execute(insertHistoryIdentitySQL, fixture.payloadID, identity)
	}
}

func observationIdentity(count int) string {
	if count == 2 {
		return regularObservationID
	}
	if count == 0 {
		return emptyObservationID
	}
	return invalidObservationID
}

func (fixture historyFixture) insert(listingID int64, count, index int, amount, currency, basis, method any) error {
	_, err := fixture.database.ExecContext(fixture.context, insertHistoryObservationSQL, fixture.payloadID,
		listingID, count, index, amount, currency, basis, method, observationIdentity(count))
	return err
}

func (fixture historyFixture) insertRegularOffers() {
	for index, amount := range []string{"24.90", "22.90"} {
		requireNoError(fixture.test, fixture.insert(fixture.listingID, 2, index, amount, "BRL", "package", "any"))
	}
}

func requireSQLCode(test *testing.T, err error, code string) {
	test.Helper()
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) || postgresError.Code != code {
		test.Fatalf("expected PostgreSQL error %s; got %v", code, err)
	}
}

func (fixture historyFixture) expectFailure(query, code string, arguments ...any) {
	fixture.test.Helper()
	_, err := fixture.database.ExecContext(fixture.context, query, arguments...)
	requireSQLCode(fixture.test, err, code)
}

func (fixture historyFixture) checkMoneyAndIdentityConstraints() {
	requireSQLCode(fixture.test, fixture.insert(fixture.listingID, 1, 0, "24.901", "BRL", "package", "any"), "23514")
	requireSQLCode(fixture.test, fixture.insert(-1, 1, 0, "24.90", "BRL", "package", "any"), "23503")
	requireSQLCode(fixture.test, fixture.insert(fixture.listingID, 0, 0, "0.00", "BRL", "package", "any"), "23514")
	fixture.expectFailure(`UPDATE price_observations SET observation_id=NULL WHERE payload_id=$1`, "23502", fixture.payloadID)
	fixture.expectFailure(`UPDATE price_observations SET scraper_id=NULL WHERE payload_id=$1`, "23502", fixture.payloadID)
	requireNoError(fixture.test, fixture.insert(fixture.listingID, 0, 0, nil, nil, nil, nil))
}

func (fixture historyFixture) checkListingConstraints() {
	fixture.expectFailure(`INSERT INTO shop_products (shop_id,source_product_id,name,product_url)
        SELECT shop_id,source_product_id,name,product_url FROM shop_products WHERE id=$1`, "23505", fixture.listingID)
	fixture.expectFailure(`UPDATE shop_products SET product_id=-1 WHERE id=$1`, "23503", fixture.listingID)
}

func (fixture historyFixture) matchProducts() int64 {
	productID := fixture.queryIdentity(canonicalHistoryProductSQL)
	fixture.execute(`UPDATE shop_products SET product_id=$1,name='Novo nome de exibição' WHERE id=$2`, productID, fixture.listingID)
	fixture.execute(`INSERT INTO shop_products (shop_id,product_id,source_product_id,name,product_url)
        VALUES ('outro-mercado',$1,'sku-000123','Arroz branco','https://outro.example.com/products/arroz')`, productID)
	fixture.assert(`SELECT count(*)=2 FROM shop_products WHERE product_id=$1`, productID)
	return productID
}

func (fixture historyFixture) checkRetention() {
	fixture.expectFailure(`UPDATE ingestion_payloads SET payload=NULL,payload_removed_at=now() WHERE id=$1`, "23514", fixture.payloadID)
	fixture.execute(`UPDATE ingestion_payloads SET status='processed',processed_at=now(),
        payload=NULL,payload_removed_at=now() WHERE id=$1`, fixture.payloadID)
	fixture.expectFailure(insertHistoryPayloadSQL, "23505", `{}`)
	fixture.expectFailure(`DELETE FROM ingestion_payloads WHERE id=$1`, "23503", fixture.payloadID)
	fixture.assert(`SELECT count(*)=3 FROM price_observations WHERE payload_id=$1`, fixture.payloadID)
}
