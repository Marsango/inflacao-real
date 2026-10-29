package database

import (
	"encoding/json"
	"os"
	"testing"

	"inflacao-real/core/internal/monitoring"
)

func TestMonitoringSeedAndEnabledScope(t *testing.T) {
	ctx, db := testDatabase(t)
	store := monitoring.PostgresStore{DB: db}
	targets, err := store.List(ctx)
	if err != nil || len(targets) != 65 {
		t.Fatalf("expected sixty-five seeded targets; got %v, %v", targets, err)
	}
	checkSeededMatchingExamples(t, targets)
	if _, err := db.ExecContext(ctx, `UPDATE monitoring_targets SET enabled=false WHERE id='cafe-moido'`); err != nil {
		t.Fatal(err)
	}
	targets, err = store.List(ctx)
	if err != nil || len(targets) != 64 {
		t.Fatalf("expected sixty-four enabled targets; got %v, %v", targets, err)
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
	assertWorkerSQL(t, requestContext, pool, `SELECT count(*)=65 FROM monitoring_targets`)
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
	assertWorkerSQL(t, requestContext, pool, `SELECT count(*)=65 FROM monitoring_targets`)
	assertWorkerSQL(t, requestContext, pool, `SELECT NOT enabled AND name='Local apple target'
		AND (criteria->'excluded_terms') ?& ARRAY['local-only', 'detergente']
		FROM monitoring_targets WHERE id='maca'`)
	assertWorkerSQL(t, requestContext, pool, `SELECT count(*)=0 FROM monitoring_targets
		WHERE id IN ('goma-tapioca','queijo-minas-frescal')`)
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
