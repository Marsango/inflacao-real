package webapi

import (
	"net/url"
	"strings"
	"testing"
)

func TestSearchValidationAndDefaults(test *testing.T) {
	store := &fakeStore{}
	assertStatus(test, store, "/api/v1/products?q=CAF%C3%89", 200)
	if store.products.Query != "CAFÉ" || store.products.Limit != 20 {
		test.Fatalf("search not forwarded: %+v", store.products)
	}
	invalid := []string{"", "a", "  ", "***", "a!", strings.Repeat("a", 101)}
	for _, search := range invalid {
		assertStatus(test, store, "/api/v1/products?q="+url.QueryEscape(search), 400)
	}
}

func seedSearchProducts(fixture fixture) {
	fixture.submit(1, nil)
	fixture.submit(2, secondPricedDay)
	fixture.execute(`UPDATE products SET name='Café torrado 500g', brand='União' WHERE id=1`)
	fixture.execute(`INSERT INTO products (name, brand) VALUES
        ('Café moído 500 g', 'União'), ('Café sem observações', 'União')`)
	fixture.execute(`UPDATE price_observations SET product_id=2 WHERE offer_index=1`)
}

func TestIndexedProductSearch(test *testing.T) {
	fixture := newFixture(test)
	seedSearchProducts(fixture)
	assertSearch(test, fixture, "CAFE uni", "1", "2")
	assertSearch(test, fixture, "uniao torr 500 g", "1")
	assertSearch(test, fixture, "uni cafe moid 500g", "2")
	assertSearch(test, fixture, "cafe | ! uniao:*", "1", "2")
	assertSearch(test, fixture, "cafe' OR '1'='1")
	assertSearch(test, fixture, "inexistente")
	assertSearch(test, fixture, "中文")
	fixture.execute(`UPDATE products SET name='Chocolate',brand=NULL WHERE id=1`)
	assertSearch(test, fixture, "choc", "1")
	assertSearch(test, fixture, "cafe", "2")
}

func assertSearch(test *testing.T, fixture fixture, search string, identities ...string) {
	test.Helper()
	var result Products
	fixture.get("/api/v1/products?q="+url.QueryEscape(search), &result)
	if len(result.Items) != len(identities) || result.Items == nil {
		test.Fatalf("search %q: %+v, expected %v", search, result, identities)
	}
	for index, identity := range identities {
		assertSearchIdentity(test, result.Items[index], identity)
	}
}

func assertSearchIdentity(test *testing.T, product Product, expected string) {
	test.Helper()
	if product.ID != expected {
		test.Fatalf("unexpected product: %+v, expected %s", product, expected)
	}
}

func TestSearchPaginationPreservesHistoricalAssignments(test *testing.T) {
	fixture := newFixture(test)
	seedSearchProducts(fixture)
	var first, second Products
	fixture.get("/api/v1/products?q=cafe&limit=1", &first)
	if len(first.Items) != 1 || first.Items[0].ID != "1" || first.NextCursor == nil {
		test.Fatalf("unexpected first search page: %+v", first)
	}
	fixture.get("/api/v1/products?q=cafe&limit=1&after="+*first.NextCursor, &second)
	if len(second.Items) != 1 || second.Items[0].ID != "2" || second.NextCursor != nil {
		test.Fatalf("unexpected second search page: %+v", second)
	}
}

func TestCatalogRequiresTwoDaysOfHistoricalInStockPrices(test *testing.T) {
	fixture := newFixture(test)
	seedSearchProducts(fixture)
	fixture.execute(`UPDATE price_observations SET availability='out_of_stock' WHERE product_id=2`)
	assertSearch(test, fixture, "cafe", "1")
	assertCatalogIdentities(test, fixture, "1")
	fixture.execute(`UPDATE price_observations SET availability='unknown' WHERE product_id=1`)
	assertSearch(test, fixture, "cafe")
	assertCatalogIdentities(test, fixture)
	fixture.execute(`UPDATE price_observations SET availability='in_stock' WHERE product_id=2`)
	assertSearch(test, fixture, "cafe", "2")
	assertCatalogIdentities(test, fixture, "2")
}

func assertCatalogIdentities(test *testing.T, fixture fixture, identities ...string) {
	test.Helper()
	var result Products
	fixture.get("/api/v1/products?limit=1", &result)
	if len(result.Items) != len(identities) || result.NextCursor != nil {
		test.Fatalf("unexpected catalog: %+v", result)
	}
	for index, identity := range identities {
		assertSearchIdentity(test, result.Items[index], identity)
	}
}

func TestPreviouslyAvailableProductsRemainSearchable(test *testing.T) {
	fixture := newFixture(test)
	fixture.submit(1, nil)
	fixture.submit(2, secondPricedDay)
	fixture.submit(3, func(observation map[string]any) {
		observation["observed_at"] = "2026-10-29T12:00:00Z"
		observation["availability"], observation["offers"] = "out_of_stock", []any{}
	})
	fixture.execute(`UPDATE shop_products SET product_id=NULL`)
	assertSearch(test, fixture, "arroz", "1")
	assertCatalogIdentities(test, fixture, "1")
}

func secondPricedDay(observation map[string]any) {
	setPrice(observation, "2026-10-28T12:00:00Z", "26.90")
}

func TestInStockWithoutAnyPriceIsNotSearchable(test *testing.T) {
	fixture := newFixture(test)
	fixture.submit(1, func(observation map[string]any) { observation["offers"] = []any{} })
	assertSearch(test, fixture, "arroz")
	assertCatalogIdentities(test, fixture)
}

func TestSearchDoesNotCountOffersOrSameDayCollectionsAsComparison(test *testing.T) {
	fixture := newFixture(test)
	fixture.submit(1, nil)
	fixture.submit(2, func(observation map[string]any) {
		setPrice(observation, "2026-10-27T22:00:00Z", "26.90")
	})
	assertSearch(test, fixture, "arroz")
	assertCatalogIdentities(test, fixture)
	fixture.submit(3, secondPricedDay)
	assertSearch(test, fixture, "arroz", "1")
	assertCatalogIdentities(test, fixture, "1")
}

func TestSearchRequiresTwoDatesInTheSameContext(test *testing.T) {
	fixture := newFixture(test)
	fixture.submit(1, nil)
	fixture.submit(2, func(observation map[string]any) {
		secondPricedDay(observation)
		observation["context"].(map[string]any)["store_id"] = "another-branch"
	})
	assertSearch(test, fixture, "arroz")
	assertCatalogIdentities(test, fixture)
}
