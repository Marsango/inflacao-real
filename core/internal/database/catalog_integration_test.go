package database

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
)

type catalogFixture struct {
	shop, sku, name, brand, gtin, quantity, unit, observedAt string
	count                                                    int
}

func enqueueCatalogFixture(test *testing.T, requestContext context.Context, pool *sql.DB, number int, fixture catalogFixture) int64 {
	test.Helper()
	raw := workerPayload(test, "submission", number, func(document map[string]any) { fixture.apply(document, number) })
	return enqueueWorkerPayload(test, requestContext, pool, raw)
}

func (fixture catalogFixture) apply(document map[string]any, number int) {
	document["shop_id"] = fixture.shop
	observation := firstObservation(document)
	observation["observation_id"] = fmt.Sprintf("00000000-0000-4000-8000-%012d", number)
	if fixture.observedAt != "" {
		observation["observed_at"] = fixture.observedAt
	}
	fixture.applyProduct(observation["product"].(map[string]any))
}

func (fixture catalogFixture) applyProduct(product map[string]any) {
	product["source_product_id"], product["name"] = fixture.sku, fixture.name
	product["brand"], product["gtin"], product["package"] = nil, nil, nil
	if fixture.brand != "" {
		product["brand"] = fixture.brand
	}
	if fixture.gtin != "" {
		product["gtin"] = fixture.gtin
	}
	if fixture.quantity != "" {
		product["package"] = map[string]any{"quantity": fixture.quantity, "unit": fixture.unit, "count": fixture.count}
	}
}

func registerCatalogShops(test *testing.T, requestContext context.Context, pool *sql.DB) {
	test.Helper()
	executeSQL(test, requestContext, pool, `INSERT INTO shops (id,name,website_url) VALUES
        ('shop-a','Shop A','https://a.example.com'),('shop-b','Shop B','https://b.example.com'),
        ('shop-c','Shop C','https://c.example.com'),('shop-d','Shop D','https://d.example.com')`)
}

func newCatalogFixture(test *testing.T) workerFixture {
	fixture := newWorkerFixture(test)
	registerCatalogShops(test, fixture.context, fixture.database)
	return fixture
}

func (fixture workerFixture) processCatalog(number int, product catalogFixture) int64 {
	identity := enqueueCatalogFixture(fixture.test, fixture.context, fixture.database, number, product)
	fixture.process()
	return identity
}

func TestCatalogMatchesAcrossShopsAndCreatesUnknownProducts(test *testing.T) {
	fixture := newCatalogFixture(test)
	product := fixture.checkAttributeMatch()
	product = fixture.checkListingEnrichment(product)
	product = fixture.checkGTINMatch(product)
	fixture.checkUnknownListings(product)
}

func (fixture workerFixture) checkAttributeMatch() catalogFixture {
	product := catalogFixture{shop: "shop-a", sku: "rice-a", name: "Arroz Branco Marca Exemplo 5kg",
		brand: "Marca Exemplo", quantity: "5", unit: "kg", count: 1}
	first := fixture.processCatalog(1, product)
	product.shop, product.sku, product.name, product.brand, product.quantity, product.unit = "shop-b", "rice-b", "MARCA EXEMPLO — BRANCO ARROZ 5000 g", "MARCA-EXEMPLO", "5000", "g"
	second := fixture.processCatalog(2, product)
	fixture.assert(`SELECT count(*)=1 FROM products`)
	fixture.assert(`SELECT count(*)=2 AND count(DISTINCT product_id)=1 FROM shop_products`)
	fixture.assert(`SELECT bool_and(product_match_method='attributes_v1') FROM price_observations WHERE payload_id=$1`, second)
	fixture.assert(`SELECT count(*)=4 AND count(DISTINCT product_id)=1 FROM price_observations WHERE payload_id IN ($1,$2)`, first, second)
	return product
}

func (fixture workerFixture) checkListingEnrichment(product catalogFixture) catalogFixture {
	product.gtin = "4006381333931"
	identity := fixture.processCatalog(3, product)
	fixture.assert(`SELECT count(*)=1 AND bool_and(gtin='04006381333931') FROM products`)
	fixture.assert(`SELECT bool_and(product_match_method='listing_v1') FROM price_observations WHERE payload_id=$1`, identity)
	return product
}

func (fixture workerFixture) checkGTINMatch(product catalogFixture) catalogFixture {
	product.shop, product.sku, product.name, product.gtin = "shop-c", "rice-c", "Pacote de arroz da Marca Exemplo", "04006381333931"
	identity := fixture.processCatalog(4, product)
	fixture.assert(`SELECT count(*)=1 FROM products`)
	fixture.assert(`SELECT bool_and(product_match_method='gtin_v1') FROM price_observations WHERE payload_id=$1`, identity)
	return product
}

func (fixture workerFixture) checkUnknownListings(product catalogFixture) {
	product.shop, product.sku, product.gtin, product.brand, product.quantity = "shop-d", "unknown", "", "", ""
	first := fixture.processCatalog(5, product)
	second := fixture.processCatalog(6, product)
	fixture.assert(`SELECT count(*)=2 FROM products`)
	fixture.assert(`SELECT count(DISTINCT product_id)=1 FROM price_observations WHERE payload_id IN ($1,$2)`, first, second)
	fixture.assert(`SELECT bool_and(product_id IS NOT NULL) FROM shop_products`)
	product.shop, product.sku = "shop-b", "another-unknown"
	fixture.processCatalog(7, product)
	fixture.assert(`SELECT count(*)=3 FROM products`)
}

func TestCatalogMatchesExistingManualProductAndEnrichesUnknownAttributes(test *testing.T) {
	fixture := newCatalogFixture(test)
	existing := fixture.queryIdentity(`INSERT INTO products (name,gtin)
        VALUES ('Previously registered product','4006381333931') RETURNING id`)
	product := catalogFixture{shop: "shop-a", sku: "rice", name: "Arroz Branco 5kg", brand: "Marca A",
		gtin: "04006381333931", quantity: "5", unit: "kg", count: 1}
	identity := fixture.processCatalog(1, product)
	fixture.assert(`SELECT count(*)=1 FROM products`)
	fixture.assert(`SELECT name='Previously registered product' AND brand='Marca A'
        AND gtin='4006381333931' AND package_quantity=5 AND package_unit='kg' AND package_count=1
        FROM products WHERE id=$1`, existing)
	fixture.assert(`SELECT bool_and(product_id=$1 AND product_match_method='gtin_v1')
        FROM price_observations WHERE payload_id=$2`, existing, identity)
}

func milkFixture() catalogFixture {
	return catalogFixture{shop: "shop-a", sku: "milk", name: "Leite Integral Marca A 1L", brand: "Marca A",
		gtin: "4006381333931", quantity: "1", unit: "l", count: 1, observedAt: "2026-10-27T12:00:00Z"}
}

func TestCatalogKeepsVariantsAndHistoricalAssignmentsSeparate(test *testing.T) {
	fixture := newCatalogFixture(test)
	product := milkFixture()
	oldIdentity := fixture.processCatalog(1, product)
	product.quantity, product.name, product.observedAt = "0.9", "Leite Integral Marca A 900ml", "2026-10-28T12:00:00Z"
	newIdentity := fixture.processCatalog(2, product)
	fixture.assertVariantAssignment(oldIdentity, newIdentity)
	product = fixture.checkLateVariant(product, oldIdentity)
	fixture.checkOtherVariants(product)
}

func (fixture workerFixture) assertVariantAssignment(oldIdentity, newIdentity int64) {
	fixture.assert(`SELECT count(*)=2 FROM products`)
	fixture.assert(`SELECT count(DISTINCT product_id)=2 FROM price_observations WHERE payload_id IN ($1,$2)`, oldIdentity, newIdentity)
	fixture.assert(`SELECT bool_and(observation.product_id=listing.product_id) FROM price_observations observation
        JOIN shop_products listing ON listing.id=observation.shop_product_id WHERE observation.payload_id=$1`, newIdentity)
	fixture.assert(`SELECT bool_and(observation.product_id<>listing.product_id) FROM price_observations observation
        JOIN shop_products listing ON listing.id=observation.shop_product_id WHERE observation.payload_id=$1`, oldIdentity)
}

func (fixture workerFixture) checkLateVariant(product catalogFixture, oldIdentity int64) catalogFixture {
	product.quantity, product.name, product.observedAt = "1", "Leite Integral Marca A 1L", "2026-10-26T12:00:00Z"
	lateIdentity := fixture.processCatalog(3, product)
	fixture.assert(`SELECT count(*)=2 FROM products`)
	fixture.assert(`SELECT count(DISTINCT product_id)=1 FROM price_observations WHERE payload_id IN ($1,$2)`, oldIdentity, lateIdentity)
	fixture.assert(`SELECT name='Leite Integral Marca A 900ml' FROM shop_products WHERE shop_id='shop-a' AND source_product_id='milk'`)
	return product
}

func variantChanges() []func(*catalogFixture) {
	return []func(*catalogFixture){
		func(product *catalogFixture) { product.gtin = "036000291452" },
		func(product *catalogFixture) { product.brand = "Marca B" },
		func(product *catalogFixture) { product.gtin, product.name = "", "Leite Desnatado Marca A 1L" },
		func(product *catalogFixture) { product.count = 6 },
	}
}

func (fixture workerFixture) checkOtherVariants(product catalogFixture) {
	for position, change := range variantChanges() {
		fixture.processVariant(product, position, change)
	}
	fixture.assert(`SELECT count(*)=6 FROM products`)
}

func (fixture workerFixture) processVariant(product catalogFixture, position int, change func(*catalogFixture)) {
	product.shop, product.sku = "shop-b", fmt.Sprintf("variant-%d", position)
	change(&product)
	fixture.processCatalog(position+10, product)
}

func TestCatalogAmbiguousCandidatesDoNotPickAnArbitraryProduct(test *testing.T) {
	fixture := newCatalogFixture(test)
	fixture.execute(`INSERT INTO products (name,brand,gtin,package_quantity,package_unit,package_count)
        SELECT 'Arroz Branco 5kg','Marca A','4006381333931',5,'kg',1 FROM generate_series(1,2)`)
	product := catalogFixture{shop: "shop-a", sku: "rice", name: "Arroz Branco 5kg", brand: "Marca A",
		gtin: "4006381333931", quantity: "5", unit: "kg", count: 1}
	identity := fixture.processCatalog(1, product)
	fixture.assert(`SELECT count(*)=3 FROM products`)
	fixture.assert(`SELECT bool_and(product_id=(SELECT max(id) FROM products)
        AND product_match_method='created_v1') FROM price_observations WHERE payload_id=$1`, identity)
	identity = fixture.processCatalog(2, product)
	fixture.assert(`SELECT count(*)=3 FROM products`)
	fixture.assert(`SELECT bool_and(product_match_method='listing_v1') FROM price_observations WHERE payload_id=$1`, identity)
}

func TestCatalogConcurrentWorkersCreateOneProductPerIdentity(test *testing.T) {
	fixture := newCatalogFixture(test)
	for group := range 2 {
		fixture.enqueueProductGroup(group)
	}
	fixture.processConcurrently(4, drainWorker)
	fixture.assert(`SELECT count(*)=8 AND bool_and(status='processed' AND attempts=1) FROM ingestion_payloads`)
	fixture.assert(`SELECT count(*)=2 FROM products`)
	fixture.assert(`SELECT count(*)=8 AND count(DISTINCT product_id)=2 FROM shop_products`)
	fixture.assert(`SELECT count(*)=16 AND count(DISTINCT product_id)=2 FROM price_observations`)
	fixture.assert(`SELECT count(*)=4 FROM price_observations WHERE product_match_method='created_v1'`)
}

func (fixture workerFixture) enqueueProductGroup(group int) {
	for position, shop := range []string{"shop-a", "shop-b", "shop-c", "shop-d"} {
		product := concurrentProduct(group, shop)
		enqueueCatalogFixture(fixture.test, fixture.context, fixture.database, group*10+position+1, product)
	}
}

func concurrentProduct(group int, shop string) catalogFixture {
	product := catalogFixture{shop: shop, sku: fmt.Sprintf("product-%d", group), name: "Café Torrado 500g",
		brand: "Marca A", quantity: "500", unit: "g", count: 1}
	if group == 0 {
		product.gtin = "4006381333931"
		return product
	}
	product.name = "Café Descafeinado 500g"
	return product
}

func TestCatalogRetainsPackageAndComponentReductionsWithLaterUnknownSize(test *testing.T) {
	test.Run("mass", func(test *testing.T) {
		fixture := newCatalogFixture(test)
		product := catalogFixture{shop: "shop-a", sku: "coffee", name: "Café Tradicional 500g",
			brand: "Marca A", quantity: "500", unit: "g", count: 1, observedAt: "2026-10-27T12:00:00Z"}
		fixture.checkSizeReduction(product, "400", 1)
	})
	test.Run("components", func(test *testing.T) {
		fixture := newCatalogFixture(test)
		product := catalogFixture{shop: "shop-a", sku: "cans", name: "Bebida lata 350ml",
			brand: "Marca A", quantity: "350", unit: "ml", count: 6, observedAt: "2026-10-27T12:00:00Z"}
		fixture.checkSizeReduction(product, "350", 5)
	})
}

func (fixture workerFixture) checkSizeReduction(product catalogFixture, quantity string, count int) {
	first := fixture.processCatalog(1, product)
	oldQuantity, oldCount := product.quantity, product.count
	product.quantity, product.count, product.observedAt = quantity, count, "2026-10-28T12:00:00Z"
	second := fixture.processCatalog(2, product)
	fixture.assertVariantAssignment(first, second)
	fixture.assertHistoricalSize(first, oldQuantity, oldCount)
	fixture.assertHistoricalSize(second, quantity, count)
	product.quantity, product.observedAt = "", "2026-10-29T12:00:00Z"
	unknown := fixture.processCatalog(3, product)
	fixture.assert(`SELECT count(*)=2 AND bool_and(package_quantity IS NULL AND package_unit IS NULL
        AND package_count IS NULL) FROM price_observations WHERE payload_id=$1`, unknown)
	fixture.assertHistoricalSize(first, oldQuantity, oldCount)
	fixture.assertHistoricalSize(second, quantity, count)
}

func (fixture workerFixture) assertHistoricalSize(identity int64, quantity string, count int) {
	fixture.assert(`SELECT count(*)=2 AND bool_and(package_quantity=$2::numeric AND package_count=$3
        AND price_basis='package') FROM price_observations WHERE payload_id=$1`, identity, quantity, count)
}
