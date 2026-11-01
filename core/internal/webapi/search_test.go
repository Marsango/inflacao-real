package webapi

import (
	"fmt"
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

func TestCuratedFiltersUseIngestionLabels(test *testing.T) {
	fixture := newFixture(test)
	names := []string{"Arroz branco tipo 1", "Banana nanica", "Bala de banana", "Peito de frango", "Sabão em pó", "Produto sem classificação"}
	for position, name := range names {
		seedLabeledProduct(fixture, position+1, name)
	}
	assertGroupProducts(fixture, "cesta-basica", "1", "2")
	assertGroupProducts(fixture, "hortifruti", "2")
	assertGroupProducts(fixture, "doces", "3")
	assertGroupProducts(fixture, "carnes", "4")
	assertGroupHistory(fixture, "cesta-basica", 2, 1500)
	assertGroupHistory(fixture, "hortifruti", 1, 2000)
	assertGroupHistory(fixture, "doces", 1, 3000)
	assertGroupHistory(fixture, "carnes", 1, 4000)
	if history := fixture.history("global", "2026-10-27", "2026-10-28"); history.Counts.Products != 6 {
		test.Fatal("unfiltered overview must include unclassified products")
	}
}

func seedLabeledProduct(fixture fixture, identity int, name string) {
	fixture.submit(identity*2, func(observation map[string]any) {
		labelProductObservation(observation, identity, name)
		setPrice(observation, "2026-10-27T12:00:00Z", "10.00")
	})
	fixture.submit(identity*2+1, func(observation map[string]any) {
		labelProductObservation(observation, identity, name)
		setPrice(observation, "2026-10-28T12:00:00Z", fmt.Sprintf("%d.00", 10+identity))
	})
}

func labelProductObservation(observation map[string]any, identity int, name string) {
	product := observation["product"].(map[string]any)
	product["source_product_id"], product["name"] = fmt.Sprint(identity), name
}

func assertGroupProducts(fixture fixture, group string, identities ...string) {
	fixture.test.Helper()
	var result Products
	fixture.get("/api/v1/products?group="+group, &result)
	if len(result.Items) != len(identities) || result.Items == nil {
		fixture.test.Fatalf("group %s: %+v, expected %v", group, result, identities)
	}
	for position, identity := range identities {
		assertSearchIdentity(fixture.test, result.Items[position], identity)
	}
}

func assertGroupHistory(fixture fixture, group string, count int, change int64) {
	fixture.test.Helper()
	history := fixture.history("global&group="+group, "2026-10-27", "2026-10-28")
	if history.Counts.Products != count || history.Counts.IncludedProducts != count || history.Unit != "percent" {
		fixture.test.Fatalf("incorrect selection: %+v", history)
	}
	assertPrice(fixture.test, history.ChangeBasisPoints, change)
	if !strings.Contains(history.Methodology, "Seleção:") || !strings.Contains(history.Title, "Variação de preços ·") {
		fixture.test.Fatal("missing selection context")
	}
}

func TestCuratedSearchPaginationAndEmptyHistory(test *testing.T) {
	fixture := newFixture(test)
	seedLabeledProduct(fixture, 1, "Banana nanica")
	seedLabeledProduct(fixture, 2, "Banana prata")
	var first, second Products
	fixture.get("/api/v1/products?group=hortifruti&q=banana&limit=1", &first)
	if len(first.Items) != 1 || first.NextCursor == nil || *first.NextCursor != "1" {
		test.Fatalf("unexpected first page: %+v", first)
	}
	fixture.get("/api/v1/products?group=hortifruti&q=banana&limit=1&after=1", &second)
	if len(second.Items) != 1 || second.Items[0].ID != "2" || second.NextCursor != nil {
		test.Fatalf("unexpected second page: %+v", second)
	}
	assertGroupProducts(fixture, "doces")
	assertEmptyGroupHistory(fixture)
}

func assertEmptyGroupHistory(fixture fixture) {
	history := fixture.history("global&group=doces", "2026-10-27", "2026-10-28")
	if len(history.Points) != 0 || history.ChangeBasisPoints != nil || history.Counts.Products != 0 || history.Title != "Variação de preços · Doces" {
		fixture.test.Fatalf("empty selection fabricated data: %+v", history)
	}
	history = fixture.history("1&group=doces", "2026-10-27", "2026-10-28")
	if len(history.Points) != 0 || history.Unit != "BRL" {
		fixture.test.Fatal("product outside selection must have empty history")
	}
}

func TestCuratedFiltersIncludeEquivalentMembersOnce(test *testing.T) {
	fixture := newFixture(test)
	seedLabeledProduct(fixture, 1, "Banana nanica")
	seedLabeledProduct(fixture, 2, "Banana prata")
	fixture.execute(`INSERT INTO product_equivalence VALUES (1,1),(2,1)`)
	fixture.execute(`DELETE FROM product_monitoring_targets WHERE product_id=1`)
	fixture.execute(`UPDATE shop_products SET product_id=NULL`)
	assertGroupProducts(fixture, "hortifruti", "1")
	assertGroupHistory(fixture, "hortifruti", 1, 1500)
	history := fixture.history("2&group=hortifruti", "2026-10-27", "2026-10-28")
	if history.Counts.Streams != 2 || history.Counts.Products != 1 {
		test.Fatal("equivalent member lost historical observations")
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
