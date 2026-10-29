package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
)

type catalogFixture struct {
	shop, sku, name, brand, gtin, quantity, unit, observedAt string
	count                                                    int
}

func enqueueCatalogFixture(t *testing.T, ctx context.Context, db *sql.DB, number int, f catalogFixture) int64 {
	t.Helper()
	return enqueueWorkerPayload(t, ctx, db, workerPayload(t, "submission", number, func(doc map[string]any) {
		doc["shop_id"] = f.shop
		o := firstObservation(doc)
		o["observation_id"] = fmt.Sprintf("00000000-0000-4000-8000-%012d", number)
		if f.observedAt != "" {
			o["observed_at"] = f.observedAt
		}
		p := o["product"].(map[string]any)
		p["source_product_id"], p["name"] = f.sku, f.name
		p["brand"], p["gtin"], p["package"] = nil, nil, nil
		if f.brand != "" {
			p["brand"] = f.brand
		}
		if f.gtin != "" {
			p["gtin"] = f.gtin
		}
		if f.quantity != "" {
			p["package"] = map[string]any{"quantity": f.quantity, "unit": f.unit, "count": f.count}
		}
	}))
}

func registerCatalogShops(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `INSERT INTO shops (id,name,website_url) VALUES
		('shop-a','Shop A','https://a.example.com'),('shop-b','Shop B','https://b.example.com'),
		('shop-c','Shop C','https://c.example.com'),('shop-d','Shop D','https://d.example.com')`); err != nil {
		t.Fatal(err)
	}
}

func TestCatalogMatchesAcrossShopsAndCreatesUnknownProducts(t *testing.T) {
	ctx, db := testDatabase(t)
	registerCatalogShops(t, ctx, db)
	w := workerForTest(t, db)
	f := catalogFixture{shop: "shop-a", sku: "rice-a", name: "Arroz Branco Marca Exemplo 5kg",
		brand: "Marca Exemplo", quantity: "5", unit: "kg", count: 1}
	id1 := enqueueCatalogFixture(t, ctx, db, 1, f)
	processWorkerPayload(t, ctx, w)
	f.shop, f.sku, f.name, f.brand, f.quantity, f.unit = "shop-b", "rice-b", "MARCA EXEMPLO — BRANCO ARROZ 5000 g", "MARCA-EXEMPLO", "5000", "g"
	id2 := enqueueCatalogFixture(t, ctx, db, 2, f)
	processWorkerPayload(t, ctx, w)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=1 FROM products`)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=2 AND count(DISTINCT product_id)=1 FROM shop_products`)
	assertWorkerSQL(t, ctx, db, `SELECT bool_and(product_match_method='attributes_v1') FROM price_observations WHERE payload_id=$1`, id2)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=4 AND count(DISTINCT product_id)=1 FROM price_observations WHERE payload_id IN ($1,$2)`, id1, id2)

	// A barcode supplied later enriches the existing product instead of requiring registration.
	f.gtin = "4006381333931"
	id3 := enqueueCatalogFixture(t, ctx, db, 3, f)
	processWorkerPayload(t, ctx, w)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=1 AND bool_and(gtin='04006381333931') FROM products`)
	assertWorkerSQL(t, ctx, db, `SELECT bool_and(product_match_method='listing_v1') FROM price_observations WHERE payload_id=$1`, id3)
	// With GTIN evidence, the source's wording can differ substantially.
	f.shop, f.sku, f.name, f.gtin = "shop-c", "rice-c", "Pacote de arroz da Marca Exemplo", "04006381333931"
	id4 := enqueueCatalogFixture(t, ctx, db, 4, f)
	processWorkerPayload(t, ctx, w)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=1 FROM products`)
	assertWorkerSQL(t, ctx, db, `SELECT bool_and(product_match_method='gtin_v1') FROM price_observations WHERE payload_id=$1`, id4)

	// New, unidentifiable listings still get products automatically; they do not
	// borrow another shop's product just because its name resembles theirs.
	f.shop, f.sku, f.gtin, f.brand, f.quantity = "shop-d", "unknown", "", "", ""
	id5 := enqueueCatalogFixture(t, ctx, db, 5, f)
	processWorkerPayload(t, ctx, w)
	id6 := enqueueCatalogFixture(t, ctx, db, 6, f)
	processWorkerPayload(t, ctx, w)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=2 FROM products`)
	assertWorkerSQL(t, ctx, db, `SELECT count(DISTINCT product_id)=1 FROM price_observations WHERE payload_id IN ($1,$2)`, id5, id6)
	assertWorkerSQL(t, ctx, db, `SELECT bool_and(product_id IS NOT NULL) FROM shop_products`)
	// Unknown identity in a second listing cannot cross-match on name alone.
	f.shop, f.sku = "shop-b", "another-unknown"
	enqueueCatalogFixture(t, ctx, db, 7, f)
	processWorkerPayload(t, ctx, w)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=3 FROM products`)
}

func TestCatalogMatchesExistingManualProductAndEnrichesUnknownAttributes(t *testing.T) {
	ctx, db := testDatabase(t)
	registerCatalogShops(t, ctx, db)
	var existingID int64
	// Canonical records from before the worker may use an unpadded GTIN and
	// lack package metadata. Their IDs and known names must remain stable.
	if err := db.QueryRowContext(ctx, `INSERT INTO products (name,gtin)
		VALUES ('Previously registered product','4006381333931') RETURNING id`).Scan(&existingID); err != nil {
		t.Fatal(err)
	}
	f := catalogFixture{shop: "shop-a", sku: "rice", name: "Arroz Branco 5kg", brand: "Marca A",
		gtin: "04006381333931", quantity: "5", unit: "kg", count: 1}
	id := enqueueCatalogFixture(t, ctx, db, 1, f)
	processWorkerPayload(t, ctx, workerForTest(t, db))
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=1 FROM products`)
	assertWorkerSQL(t, ctx, db, `SELECT name='Previously registered product' AND brand='Marca A'
		AND gtin='4006381333931' AND package_quantity=5 AND package_unit='kg' AND package_count=1
		FROM products WHERE id=$1`, existingID)
	assertWorkerSQL(t, ctx, db, `SELECT bool_and(product_id=$1 AND product_match_method='gtin_v1')
		FROM price_observations WHERE payload_id=$2`, existingID, id)
}

func TestCatalogKeepsVariantsAndHistoricalAssignmentsSeparate(t *testing.T) {
	ctx, db := testDatabase(t)
	registerCatalogShops(t, ctx, db)
	w := workerForTest(t, db)
	f := catalogFixture{shop: "shop-a", sku: "milk", name: "Leite Integral Marca A 1L",
		brand: "Marca A", gtin: "4006381333931", quantity: "1", unit: "l", count: 1,
		observedAt: "2026-10-27T12:00:00Z"}
	oldID := enqueueCatalogFixture(t, ctx, db, 1, f)
	processWorkerPayload(t, ctx, w)
	// Even with the same barcode, contradictory package sizes cannot be merged.
	f.quantity, f.name, f.observedAt = "0.9", "Leite Integral Marca A 900ml", "2026-10-28T12:00:00Z"
	newID := enqueueCatalogFixture(t, ctx, db, 2, f)
	processWorkerPayload(t, ctx, w)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=2 FROM products`)
	assertWorkerSQL(t, ctx, db, `SELECT count(DISTINCT product_id)=2 FROM price_observations WHERE payload_id IN ($1,$2)`, oldID, newID)
	assertWorkerSQL(t, ctx, db, `SELECT bool_and(o.product_id=s.product_id) FROM price_observations o
		JOIN shop_products s ON s.id=o.shop_product_id WHERE o.payload_id=$1`, newID)
	assertWorkerSQL(t, ctx, db, `SELECT bool_and(o.product_id<>s.product_id) FROM price_observations o
		JOIN shop_products s ON s.id=o.shop_product_id WHERE o.payload_id=$1`, oldID)

	// A late old variant attaches to its original product without changing today's listing.
	f.quantity, f.name, f.observedAt = "1", "Leite Integral Marca A 1L", "2026-10-26T12:00:00Z"
	lateID := enqueueCatalogFixture(t, ctx, db, 3, f)
	processWorkerPayload(t, ctx, w)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=2 FROM products`)
	assertWorkerSQL(t, ctx, db, `SELECT count(DISTINCT product_id)=1 FROM price_observations WHERE payload_id IN ($1,$2)`, oldID, lateID)
	assertWorkerSQL(t, ctx, db, `SELECT name='Leite Integral Marca A 900ml' FROM shop_products WHERE shop_id='shop-a' AND source_product_id='milk'`)

	// Conflicting GTIN, brand, variant tokens and pack count each keep products apart.
	for i, change := range []func(*catalogFixture){
		func(f *catalogFixture) { f.gtin = "036000291452" },
		func(f *catalogFixture) { f.brand = "Marca B" },
		func(f *catalogFixture) { f.gtin, f.name = "", "Leite Desnatado Marca A 1L" },
		func(f *catalogFixture) { f.count = 6 },
	} {
		other := f
		other.shop, other.sku = "shop-b", fmt.Sprintf("variant-%d", i)
		change(&other)
		enqueueCatalogFixture(t, ctx, db, i+10, other)
		processWorkerPayload(t, ctx, w)
	}
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=6 FROM products`)
}

func TestCatalogAmbiguousCandidatesDoNotPickAnArbitraryProduct(t *testing.T) {
	ctx, db := testDatabase(t)
	registerCatalogShops(t, ctx, db)
	// Existing/manual products may already contain duplicates. They remain intact.
	if _, err := db.ExecContext(ctx, `INSERT INTO products (name,brand,gtin,package_quantity,package_unit,package_count)
		SELECT 'Arroz Branco 5kg','Marca A','4006381333931',5,'kg',1 FROM generate_series(1,2)`); err != nil {
		t.Fatal(err)
	}
	w := workerForTest(t, db)
	f := catalogFixture{shop: "shop-a", sku: "rice", name: "Arroz Branco 5kg", brand: "Marca A",
		gtin: "4006381333931", quantity: "5", unit: "kg", count: 1}
	id := enqueueCatalogFixture(t, ctx, db, 1, f)
	processWorkerPayload(t, ctx, w)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=3 FROM products`)
	assertWorkerSQL(t, ctx, db, `SELECT bool_and(product_id=(SELECT max(id) FROM products)
		AND product_match_method='created_v1') FROM price_observations WHERE payload_id=$1`, id)
	// The same shop/SKU can consistently reuse its own product on subsequent collections.
	id = enqueueCatalogFixture(t, ctx, db, 2, f)
	processWorkerPayload(t, ctx, w)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=3 FROM products`)
	assertWorkerSQL(t, ctx, db, `SELECT bool_and(product_match_method='listing_v1') FROM price_observations WHERE payload_id=$1`, id)
}

func TestCatalogConcurrentWorkersCreateOneProductPerIdentity(t *testing.T) {
	ctx, db := testDatabase(t)
	registerCatalogShops(t, ctx, db)
	for group := range 2 {
		for i, shop := range []string{"shop-a", "shop-b", "shop-c", "shop-d"} {
			f := catalogFixture{shop: shop, sku: fmt.Sprintf("product-%d", group), name: "Café Torrado 500g",
				brand: "Marca A", quantity: "500", unit: "g", count: 1}
			if group == 0 {
				f.gtin = "4006381333931"
			} else {
				f.name = "Café Descafeinado 500g"
			}
			enqueueCatalogFixture(t, ctx, db, group*10+i+1, f)
		}
	}
	var wg sync.WaitGroup
	results := make(chan error, 4)
	for range 4 {
		w := workerForTest(t, db)
		wg.Go(func() {
			for {
				found, err := w.ProcessNext(ctx)
				if err != nil || !found {
					results <- err
					return
				}
			}
		})
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=8 AND bool_and(status='processed' AND attempts=1) FROM ingestion_payloads`)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=2 FROM products`)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=8 AND count(DISTINCT product_id)=2 FROM shop_products`)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=16 AND count(DISTINCT product_id)=2 FROM price_observations`)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=4 FROM price_observations WHERE product_match_method='created_v1'`)
}
