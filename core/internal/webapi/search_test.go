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
