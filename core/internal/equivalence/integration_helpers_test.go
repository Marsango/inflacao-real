package equivalence

import (
	"context"
	"database/sql"
	"testing"

	"inflacao-real/core/internal/database"
	"inflacao-real/core/internal/testdatabase"
)

type fixture struct {
	test           *testing.T
	requestContext context.Context
	database       *sql.DB
}

func newFixture(test *testing.T) fixture {
	requestContext, pool := testdatabase.New(test, database.Open, database.Migrate)
	fixture := fixture{test, requestContext, pool}
	fixture.execute(`INSERT INTO products(name,brand,gtin) VALUES('Banana Maçã Kg','Hortifruti','12345670'),('Banana Maca Kg',NULL,'12345671'),('Banana Maçã Kg',NULL,'12345672')`)
	return fixture
}
func (fixture fixture) execute(query string, args ...any) {
	fixture.test.Helper()
	if _, err := fixture.database.ExecContext(fixture.requestContext, query, args...); err != nil {
		fixture.test.Fatal(err)
	}
}
func (fixture fixture) snapshot() Snapshot {
	fixture.test.Helper()
	snapshot, err := Read(fixture.requestContext, fixture.database)
	if err != nil {
		fixture.test.Fatal(err)
	}
	return snapshot
}
func (fixture fixture) apply(action string, first, second int64) error {
	fixture.test.Helper()
	decision := Decision{Revision: fixture.snapshot().Revision, Action: action, First: first, Second: second, Reason: "Conferência nas fontes", SourceVerified: true}
	return Apply(fixture.requestContext, fixture.database, "owner", decision)
}
func (fixture fixture) require(err error) {
	fixture.test.Helper()
	if err != nil {
		fixture.test.Fatal(err)
	}
}
func (fixture fixture) seedLooseEvidence() {
	fixture.execute(`INSERT INTO shops VALUES('market','Market','https://example.com')`)
	fixture.execute(`INSERT INTO shop_products(shop_id,product_id,source_product_id,name,product_url) SELECT 'market',id,id::text,name,'https://example.com/'||id FROM products`)
	fixture.execute(`INSERT INTO ingestion_payloads(scraper_id,submission_id,shop_id,schema_version,created_at,payload) VALUES('test','00000000-0000-4000-8000-000000000001','market','1.0',now(),'{}')`)
	fixture.execute(`INSERT INTO observation_identities(scraper_id,observation_id,fingerprint,payload_id,run_id,collector_name,collector_version)
 SELECT 'test',('00000000-0000-4000-8000-'||lpad(id::text,12,'0'))::uuid,decode(repeat('00',32),'hex'),1,'00000000-0000-4000-8000-000000000001','test','1' FROM shop_products`)
	fixture.execute(`INSERT INTO price_observations(payload_id,shop_product_id,observed_at,source_url,channel,availability,offer_count,offer_index,price_amount,price_currency,price_basis,payment_method,product_id,product_match_method,scraper_id,observation_id,collection_method)
 SELECT 1,id,now(),'https://example.com','online','in_stock',1,0,5,'BRL','kg','any',product_id,'created_v1','test',('00000000-0000-4000-8000-'||lpad(id::text,12,'0'))::uuid,'api' FROM shop_products`)
}
