package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"

	"inflacao-real/core/internal/monitoring"
)

func TestMonitoringSeedAndEnabledScope(t *testing.T) {
	ctx, db := testDatabase(t)
	store := monitoring.PostgresStore{DB: db}
	targets, err := store.List(ctx)
	if err != nil || len(targets) != 96 {
		t.Fatalf("expected ninety-six seeded targets; got %v, %v", targets, err)
	}
	checkSeededMatchingExamples(t, targets)
	if _, err := db.ExecContext(ctx, `UPDATE monitoring_targets SET enabled=false WHERE id='cafe-moido'`); err != nil {
		t.Fatal(err)
	}
	targets, err = store.List(ctx)
	if err != nil || len(targets) != 95 {
		t.Fatalf("expected ninety-five enabled targets; got %v, %v", targets, err)
	}
}

func checkSeededMatchingExamples(t *testing.T, targets []monitoring.Target) {
	t.Helper()
	data, err := os.ReadFile("../../../docs/contracts/monitoring/v1/matching-examples.json")
	if err != nil {
		t.Fatal(err)
	}
	var examples []struct{ Target monitoring.Target }
	if err := json.Unmarshal(data, &examples); err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]monitoring.Target)
	for _, target := range targets {
		byID[target.ID] = target
	}
	for _, example := range examples {
		checkSeededTarget(t, byID[example.Target.ID], example.Target)
	}
}

func TestEverydayFoodsMigrationPreservesExistingScope(t *testing.T) {
	requestContext, pool := testDatabase(t)
	provider, err := migrationProvider(pool)
	requireNoError(t, err)
	_, err = provider.DownTo(requestContext, 6)
	requireNoError(t, err)
	assertWorkerSQL(t, requestContext, pool, `SELECT count(*)=16 FROM monitoring_targets`)
	executeSQL(t, requestContext, pool, `UPDATE monitoring_targets
		SET enabled=false, name='Local coffee target' WHERE id='cafe-moido'`)
	requireNoError(t, Migrate(requestContext, pool))
	assertWorkerSQL(t, requestContext, pool, `SELECT count(*)=96 FROM monitoring_targets`)
	assertWorkerSQL(t, requestContext, pool, `SELECT NOT enabled AND name='Local coffee target'
		FROM monitoring_targets WHERE id='cafe-moido'`)
}

func TestHouseholdMigrationPreservesExistingScope(t *testing.T) {
	requestContext, pool := testDatabase(t)
	provider, err := migrationProvider(pool)
	requireNoError(t, err)
	_, err = provider.DownTo(requestContext, 7)
	requireNoError(t, err)
	assertWorkerSQL(t, requestContext, pool, `SELECT count(*)=37 FROM monitoring_targets`)
	executeSQL(t, requestContext, pool, `UPDATE monitoring_targets SET enabled=false,
		name='Local apple target', criteria=jsonb_set(criteria, '{excluded_terms}',
		(criteria->'excluded_terms') || '["local-only"]'::jsonb) WHERE id='maca'`)
	requireNoError(t, Migrate(requestContext, pool))
	assertWorkerSQL(t, requestContext, pool, `SELECT count(*)=96 FROM monitoring_targets`)
	assertWorkerSQL(t, requestContext, pool, `SELECT NOT enabled AND name='Local apple target'
		AND (criteria->'excluded_terms') ?& ARRAY['local-only', 'detergente']
		FROM monitoring_targets WHERE id='maca'`)
	assertWorkerSQL(t, requestContext, pool, `SELECT count(*)=0 FROM monitoring_targets
		WHERE id IN ('goma-tapioca','queijo-minas-frescal')`)
}

func TestFamilyEssentialsMigrationPreservesExistingScope(t *testing.T) {
	requestContext, pool := testDatabase(t)
	provider, err := migrationProvider(pool)
	requireNoError(t, err)
	_, err = provider.DownTo(requestContext, 8)
	requireNoError(t, err)
	assertWorkerSQL(t, requestContext, pool, `SELECT count(*)=65 FROM monitoring_targets`)
	executeSQL(t, requestContext, pool, `UPDATE monitoring_targets SET enabled=false,
		name='Local banana target', criteria=jsonb_set(criteria, '{excluded_terms}',
		(criteria->'excluded_terms') || '["local-only"]'::jsonb) WHERE id='banana'`)
	requireNoError(t, Migrate(requestContext, pool))
	assertWorkerSQL(t, requestContext, pool, `SELECT count(*)=96 FROM monitoring_targets`)
	assertWorkerSQL(t, requestContext, pool, `SELECT NOT enabled AND name='Local banana target'
		AND (criteria->'excluded_terms') ?& ARRAY['local-only', 'shampoo', 'sabao']
		FROM monitoring_targets WHERE id='banana'`)
	checkFamilyEssentialsRollback(t, requestContext, pool)
}

func checkFamilyEssentialsRollback(t *testing.T, requestContext context.Context, pool *sql.DB) {
	t.Helper()
	executeSQL(t, requestContext, pool, `WITH product AS (
		INSERT INTO products (name) VALUES ('Migration test product') RETURNING id)
		INSERT INTO product_monitoring_targets (product_id, target_id)
		SELECT id, target FROM product CROSS JOIN unnest(ARRAY['banana','fralda-infantil']) AS target`)
	provider, err := migrationProvider(pool)
	requireNoError(t, err)
	_, err = provider.DownTo(requestContext, 8)
	requireNoError(t, err)
	assertWorkerSQL(t, requestContext, pool, `SELECT count(*)=65 FROM monitoring_targets`)
	assertWorkerSQL(t, requestContext, pool, `SELECT count(*)=1 AND bool_and(target_id='banana')
		FROM product_monitoring_targets`)
	assertWorkerSQL(t, requestContext, pool, `SELECT NOT enabled AND name='Local banana target'
		AND (criteria->'excluded_terms') ? 'local-only'
		AND NOT (criteria->'excluded_terms') ?| ARRAY['shampoo','sabao']
		FROM monitoring_targets WHERE id='banana'`)
	requireNoError(t, Migrate(requestContext, pool))
}

func TestSnacksMigrationPreservesExistingScope(t *testing.T) {
	requestContext, pool := testDatabase(t)
	provider, err := migrationProvider(pool)
	requireNoError(t, err)
	_, err = provider.DownTo(requestContext, 9)
	requireNoError(t, err)
	assertWorkerSQL(t, requestContext, pool, `SELECT count(*)=77 FROM monitoring_targets`)
	executeSQL(t, requestContext, pool, `UPDATE monitoring_targets SET enabled=false,
		name='Local milk target', criteria=jsonb_set(criteria, '{excluded_terms}',
		(criteria->'excluded_terms') || '["local-only"]'::jsonb) WHERE id='leite'`)
	requireNoError(t, Migrate(requestContext, pool))
	assertWorkerSQL(t, requestContext, pool, `SELECT count(*)=96 FROM monitoring_targets`)
	assertWorkerSQL(t, requestContext, pool, `SELECT NOT enabled AND name='Local milk target'
		AND (criteria->'excluded_terms') ?& ARRAY['local-only','condensado','sorvete']
		FROM monitoring_targets WHERE id='leite'`)
	checkSnacksRollback(t, requestContext, pool)
}

func checkSnacksRollback(t *testing.T, requestContext context.Context, pool *sql.DB) {
	t.Helper()
	executeSQL(t, requestContext, pool, `WITH product AS (
		INSERT INTO products (name) VALUES ('Migration snack product') RETURNING id)
		INSERT INTO product_monitoring_targets (product_id, target_id)
		SELECT id, target FROM product CROSS JOIN unnest(ARRAY['leite','linguica']) AS target`)
	provider, err := migrationProvider(pool)
	requireNoError(t, err)
	_, err = provider.DownTo(requestContext, 9)
	requireNoError(t, err)
	assertWorkerSQL(t, requestContext, pool, `SELECT count(*)=77 FROM monitoring_targets`)
	assertWorkerSQL(t, requestContext, pool, `SELECT count(*)=1 AND bool_and(target_id='leite')
		FROM product_monitoring_targets`)
	assertWorkerSQL(t, requestContext, pool, `SELECT NOT enabled AND name='Local milk target'
		AND (criteria->'excluded_terms') ?& ARRAY['local-only','condensado','shampoo']
		AND NOT (criteria->'excluded_terms') ? 'sorvete' FROM monitoring_targets WHERE id='leite'`)
	requireNoError(t, Migrate(requestContext, pool))
}

func checkSeededTarget(t *testing.T, actual, expected monitoring.Target) {
	t.Helper()
	actualJSON, _ := json.Marshal(actual)
	expectedJSON, _ := json.Marshal(expected)
	if string(actualJSON) != string(expectedJSON) {
		t.Errorf("seed differs from contract example: %s vs %s", actualJSON, expectedJSON)
	}
}

func TestMonitoringGroupsBrandsWithoutMergingProducts(t *testing.T) {
	ctx, db := testDatabase(t)
	registerCatalogShops(t, ctx, db)
	worker := workerForTest(t, db)
	fixture := catalogFixture{shop: "shop-a", sku: "coffee-a", name: "Café Moído Marca A 500g",
		brand: "Marca A", quantity: "500", unit: "g", count: 1}
	enqueueCatalogFixture(t, ctx, db, 1, fixture)
	processWorkerPayload(t, ctx, worker)
	fixture.shop, fixture.sku, fixture.brand, fixture.name = "shop-b", "coffee-b", "Marca B", "Café Moído Marca B 500g"
	enqueueCatalogFixture(t, ctx, db, 2, fixture)
	processWorkerPayload(t, ctx, worker)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=2 FROM products`)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=2 AND count(DISTINCT product_id)=2
		FROM product_monitoring_targets WHERE target_id='cafe-moido'`)
	assertWorkerSQL(t, ctx, db, `SELECT count(DISTINCT product_id)=2 FROM price_observations`)
}

func TestProductLabelBackfillOnExistingCatalog(t *testing.T) {
	requestContext, pool := testDatabase(t)
	provider, err := migrationProvider(pool)
	requireNoError(t, err)
	_, err = provider.DownTo(requestContext, 12)
	requireNoError(t, err)
	seedUnlabeledCatalog(t, requestContext, pool)
	requireNoError(t, Migrate(requestContext, pool))
	assertWorkerSQL(t, requestContext, pool, `SELECT count(*)=1 FROM product_monitoring_targets WHERE product_id=1 AND target_id='cafe-moido'`)
	assertWorkerSQL(t, requestContext, pool, `SELECT count(*)=1 FROM product_monitoring_targets WHERE product_id=2 AND target_id='bala'`)
	assertWorkerSQL(t, requestContext, pool, `SELECT count(*)=1 FROM product_monitoring_targets WHERE product_id=3 AND target_id='banana'`)
	assertWorkerSQL(t, requestContext, pool, `SELECT count(*)=0 FROM product_monitoring_targets WHERE product_id IN (4,5)`)
	assertWorkerSQL(t, requestContext, pool, `SELECT count(*)=0 FROM product_monitoring_targets WHERE product_id=2 AND target_id='banana'`)
	requireNoError(t, Migrate(requestContext, pool))
	assertWorkerSQL(t, requestContext, pool, `SELECT count(*)=3 FROM product_monitoring_targets`)
}

func seedUnlabeledCatalog(t *testing.T, requestContext context.Context, pool *sql.DB) {
	registerCatalogShops(t, requestContext, pool)
	executeSQL(t, requestContext, pool, `INSERT INTO products (name) VALUES
        ('CAFÉ MOÍDO 500g'), ('Bala de banana'), ('Produto do catálogo'), ('Maçã gala'), ('Produto desconhecido')`)
	executeSQL(t, requestContext, pool, `INSERT INTO shop_products (shop_id,source_product_id,product_id,name,product_url)
        VALUES ('shop-a','banana',3,'Banana nanica','https://example.com/banana')`)
	executeSQL(t, requestContext, pool, `UPDATE monitoring_targets SET enabled=false WHERE id='maca'`)
	executeSQL(t, requestContext, pool, `INSERT INTO product_monitoring_targets VALUES (1,'cafe-moido')`)
}
