package webapi

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"inflacao-real/core/internal/equivalence"
	"inflacao-real/core/internal/ingestion"
	"inflacao-real/core/internal/insights"
)

func newMamboFixture(test *testing.T) fixture {
	fixture := newFixture(test)
	fixture.execute(`INSERT INTO shops VALUES ('mambo','Mambo','https://www.mambo.com.br')`)
	router, err := ingestion.NewRouter(ingestion.PostgresStore{DB: fixture.pool}, ingestion.Auth{
		Token: "test", ScraperID: "collector-sp-01", ShopIDs: []string{"mambo"}})
	requireSuccess(test, err)
	RegisterRoutes(router, PostgresStore{DB: fixture.pool})
	fixture.router = router
	return fixture
}

func (fixture fixture) submitMambo(number int, observedAt string) {
	fixture.submitStoreSource(number, observedAt, "mambo-submission")
}

func (fixture fixture) submitStoreSource(number int, observedAt, name string) {
	fixture.test.Helper()
	raw, err := os.ReadFile("../../../scrapers/tests/fixtures/stores/" + name + ".json")
	requireSuccess(fixture.test, err)
	var document map[string]any
	requireSuccess(fixture.test, json.Unmarshal(raw, &document))
	document["submission_id"] = fmt.Sprintf("00000000-0000-4000-8000-%012d", number)
	replayStoreObservations(document["observations"].([]any), number, observedAt)
	raw, err = json.Marshal(document)
	requireSuccess(fixture.test, err)
	fixture.ingest(raw)
}

func replayStoreObservations(observations []any, number int, observedAt string) {
	for position, value := range observations {
		observation := value.(map[string]any)
		observation["observation_id"] = fmt.Sprintf("00000000-0000-4000-8000-%012d", number*100+position)
		observation["observed_at"] = observedAt
	}
}

func TestMamboKilogramPricesSurviveIngestionAndHistory(test *testing.T) {
	fixture := newMamboFixture(test)
	fixture.submitStoreSource(1, "2026-10-31T12:00:00Z", "mambo-weight-submission")
	fixture.submitStoreSource(2, "2026-11-01T12:00:00Z", "mambo-weight-submission")
	var products Products
	fixture.get("/api/v1/products", &products)
	if len(products.Items) != 5 {
		test.Fatalf("expected five weight listings with two dates: %+v", products)
	}
	for _, product := range products.Items {
		assertMamboWeightHistory(fixture, product)
	}
	assertMamboWeightPersistence(fixture)
}

func assertMamboWeightHistory(fixture fixture, product Product) {
	fixture.test.Helper()
	history := fixture.history(product.ID, "2026-10-31", "2026-11-01")
	if history.PriceBasis == nil || *history.PriceBasis != "kg" || len(history.ShopPrices) != 1 {
		fixture.test.Fatalf("lost kilogram basis or retailer: %+v", history)
	}
	assertPrice(fixture.test, history.ChangeBasisPoints, 0)
	expected := map[string]int64{"Banana Nanica Kg": 999, "Iscas de Patinho Resfriado Kg": 7998,
		"Pão Francês Mambo Kg": 1990, "Melão Dino Rei Kg": 1298, "Melão Dino Kg": 1498}
	amount, found := expected[history.Title]
	if !found {
		fixture.test.Fatalf("unexpected weight product: %s", history.Title)
	}
	assertPrice(fixture.test, history.LatestPriceMinor, amount)
	assertPrice(fixture.test, history.Points[0].Value, amount)
}

func assertMamboWeightPersistence(fixture fixture) {
	fixture.test.Helper()
	pool := fixture.pool
	row := pool.QueryRowContext(fixture.ctx, `SELECT count(*)=10 AND bool_and(
		package_quantity IS NULL AND price_basis='kg' AND offer_count=1 AND postal_code='01310200'
		AND city='São Paulo' AND state='SP' AND channel='delivery' AND seller_id='1'
		AND store_id LIKE '%/seller:mambodelivery215') FROM price_observations`)
	var preserved bool
	requireSuccess(fixture.test, row.Scan(&preserved))
	if !preserved {
		fixture.test.Fatal("kilogram prices became fixed packages or estimated payment totals")
	}
}

func TestCapturedMamboIngestionToSearchableSaoPauloHistory(test *testing.T) {
	fixture := newMamboFixture(test)
	fixture.submitMambo(1, "2026-10-31T12:00:00Z")
	var products Products
	fixture.get("/api/v1/products", &products)
	if len(products.Items) != 0 {
		test.Fatal("one collection date must not qualify a product for search")
	}
	fixture.submitMambo(2, "2026-11-01T12:00:00Z")
	fixture.get("/api/v1/products", &products)
	if len(products.Items) != 1 {
		test.Fatalf("expected searchable Mambo product: %v", products)
	}
	history := fixture.history(products.Items[0].ID, "2026-10-31", "2026-11-01")
	assertMamboHistory(test, history)
	assertMamboProvenance(fixture)
}

func assertMamboHistory(test *testing.T, history insights.History) {
	test.Helper()
	assertPrice(test, history.LatestPriceMinor, 3498)
	assertPrice(test, history.ChangeBasisPoints, 0)
	if !strings.Contains(history.Coverage, "São Paulo/SP") || len(history.ShopPrices) != 1 {
		test.Fatalf("missing São Paulo coverage: %+v", history)
	}
	shop := history.ShopPrices[0]
	if shop.ShopID != "mambo" || shop.ShopName != "Mambo" || !strings.Contains(shop.Context, "01310200") {
		test.Fatalf("incorrect Mambo presentation: %+v", shop)
	}
}

func assertMamboProvenance(fixture fixture) {
	fixture.test.Helper()
	var preserved bool
	pool := fixture.pool
	row := pool.QueryRowContext(fixture.ctx, `SELECT count(*) > 0 AND bool_and(
		observation.city='São Paulo' AND observation.state='SP' AND postal_code='01310200'
		AND seller_id='1' AND store_id LIKE '%/seller:mambodelivery215' AND channel='delivery'
		AND listing.source_product_id='288357' AND identity.collector_name='mambo'
		AND source_url LIKE 'https://www.mambo.com.br/api/catalog_system/pub/products/search%')
		FROM price_observations observation JOIN shop_products listing ON listing.id=shop_product_id
		JOIN observation_identities identity USING (scraper_id, observation_id)`)
	err := row.Scan(&preserved)
	requireSuccess(fixture.test, err)
	if !preserved {
		fixture.test.Fatal("Mambo provenance was lost during ingestion")
	}
}

func TestGlobalCoveragePreservesSeparateCuritibaAndSaoPauloContexts(test *testing.T) {
	fixture := newFixture(test)
	fixture.submit(1, func(observation map[string]any) { setObservationCity(observation, "Curitiba", "PR") })
	fixture.submit(2, func(observation map[string]any) {
		setPrice(observation, "2026-10-29T12:00:00Z", "26.90")
		setObservationCity(observation, "São Paulo", "SP")
	})
	history := fixture.history("global", "2026-10-27", "2026-10-29")
	if history.ChangeBasisPoints != nil || !strings.Contains(history.Coverage, "Curitiba/PR; São Paulo/SP") {
		test.Fatalf("geographic differences became inflation or lost coverage: %+v", history)
	}
}

func setObservationCity(observation map[string]any, city, state string) {
	context := observation["context"].(map[string]any)
	context["city"], context["state"] = city, state
}

func TestIngestionToPublicHistory(test *testing.T) {
	fixture := newFixture(test)
	fixture.submit(1, nil)
	fixture.submit(2, func(observation map[string]any) { setPrice(observation, "2026-10-29T12:00:00Z", "26.90") })
	var products Products
	fixture.get("/api/v1/products", &products)
	if len(products.Items) != 1 || products.NextCursor != nil {
		test.Fatalf("unexpected products: %v", products)
	}
	history := fixture.history(products.Items[0].ID, "2026-10-27", "2026-10-29")
	assertPrice(test, history.LatestPriceMinor, 2690)
	assertPrice(test, history.ChangeBasisPoints, 803)
	assertPrice(test, history.Points[0].Value, 2490)
	checkMissingCoverage(test, history)
	global := fixture.history("global", "2026-10-27", "2026-10-29")
	assertPrice(test, global.ChangeBasisPoints, 803)
}

func TestDailyLatestMeasurementAndSaoPauloBoundaries(test *testing.T) {
	fixture := newFixture(test)
	fixture.submit(1, func(observation map[string]any) { setPrice(observation, "2026-10-28T02:59:59Z", "25.00") })
	fixture.submit(2, func(observation map[string]any) { setPrice(observation, "2026-10-28T03:00:00Z", "26.00") })
	fixture.submit(3, func(observation map[string]any) { setPrice(observation, "2026-10-28T22:00:00Z", "27.00") })
	history := fixture.history("1", "2026-10-27", "2026-10-28")
	assertPrice(test, history.Points[0].Value, 2500)
	assertPrice(test, history.Points[1].Value, 2700)
	fixture.submit(4, func(observation map[string]any) {
		observation["observed_at"], observation["availability"], observation["offers"] = "2026-10-29T01:00:00Z", "out_of_stock", []any{}
	})
	history = fixture.history("1", "2026-10-28", "2026-10-28")
	if history.Points[0].Value != nil || history.ChangeBasisPoints != nil {
		test.Fatal("unavailable latest observation reused an older price")
	}
}

func TestHistoryExcludesConditionalAndIncomparableOffers(test *testing.T) {
	fixture := newFixture(test)
	fixture.submit(1, nil)
	fixture.execute(`DELETE FROM price_observations WHERE offer_index=1`)
	changes := []string{"membership_required=true", "coupon_required=true",
		"minimum_quantity=2", "payment_method='pix'",
		"availability='unknown'", "price_basis='kg'", "package_quantity=4", "package_count=2"}
	for _, change := range changes {
		checkExcludedOffer(test, fixture, change)
	}
	fixture.execute(`UPDATE price_observations SET price_amount=0`)
	history := fixture.history("1", "2026-10-27", "2026-10-27")
	assertPrice(test, history.LatestPriceMinor, 0)
}

func checkExcludedOffer(test *testing.T, fixture fixture, change string) {
	test.Helper()
	fixture.execute(`UPDATE price_observations SET ` + change)
	history := fixture.history("1", "2026-10-27", "2026-10-27")
	if history.LatestPriceMinor != nil || history.UnknownConditions {
		test.Fatalf("incomparable offer included or misrepresented as unknown: %s", change)
	}
	fixture.execute(`UPDATE price_observations SET membership_required=false,coupon_required=false,minimum_quantity=1,
  payment_method='any',availability='in_stock',price_basis='package',package_quantity=5,package_count=1`)
}

func TestFixedContextsAndEquivalentPackages(test *testing.T) {
	fixture := newFixture(test)
	fixture.submit(1, nil)
	fixture.submit(2, func(observation map[string]any) {
		setPrice(observation, "2026-10-28T12:00:00Z", "28.90")
		observation["context"].(map[string]any)["store_id"] = "another-branch"
	})
	fixture.submit(3, func(observation map[string]any) { setPrice(observation, "2026-10-28T12:00:00Z", "24.90") })
	fixture.execute(`UPDATE price_observations SET package_quantity=5000,package_unit='g'`)
	history := fixture.history("1", "2026-10-27", "2026-10-28")
	if history.Counts.IncludedStreams != 1 || history.Counts.Streams != 2 {
		test.Fatal("panel changed between days")
	}
	assertPrice(test, history.Points[1].Value, 2490)
	if history.ChangeBasisPoints == nil || *history.ChangeBasisPoints != 0 {
		test.Fatal("unmatched branch affected variation")
	}
}

func TestPaginationAndHistoricalProductAssignment(test *testing.T) {
	fixture := newFixture(test)
	fixture.submit(1, nil)
	fixture.submit(2, secondPricedDay)
	fixture.execute(`INSERT INTO products (name) VALUES ('Another product'), ('Unobserved product')`)
	fixture.execute(`UPDATE shop_products SET product_id=2`)
	fixture.submit(3, anotherProduct)
	fixture.submit(4, func(observation map[string]any) {
		anotherProduct(observation)
		observation["observed_at"] = "2026-10-29T12:00:00Z"
	})
	history := fixture.history("1", "2026-10-27", "2026-10-27")
	assertPrice(test, history.LatestPriceMinor, 2490)
	checkCatalogPages(test, fixture)
}

func anotherProduct(observation map[string]any) {
	setPrice(observation, "2026-10-28T12:00:00Z", "10.00")
	product := observation["product"].(map[string]any)
	product["name"], product["brand"], product["gtin"], product["package"] = "Another product", nil, nil, nil
}

func TestEmptyStoreAndUnknownProducts(test *testing.T) {
	fixture := newFixture(test)
	var products Products
	fixture.get("/api/v1/products", &products)
	if products.Items == nil || len(products.Items) != 0 {
		test.Fatal("empty catalog must be an empty array")
	}
	history := fixture.history("global", "2026-10-27", "2026-10-28")
	if history.Points == nil || history.CollectedAt != nil || history.ChangeBasisPoints != nil {
		test.Fatal("empty history invented data")
	}
	assertStatus(test, PostgresStore{DB: fixture.pool}, "/api/v1/history?product=999&start=2026-10-27&end=2026-10-28", 404)
}

func checkCatalogPages(test *testing.T, fixture fixture) {
	test.Helper()
	var first, second Products
	fixture.get("/api/v1/products?limit=1", &first)
	if first.NextCursor == nil {
		test.Fatal("missing next cursor")
	}
	fixture.get("/api/v1/products?limit=1&after="+*first.NextCursor, &second)
	if len(second.Items) != 1 || first.Items[0].ID == second.Items[0].ID || second.NextCursor != nil {
		test.Fatal("pagination lost historical products or included unobserved products")
	}
}

func TestExplicitUnitPricesAndAmbiguousBases(test *testing.T) {
	fixture := newFixture(test)
	fixture.submit(1, func(observation map[string]any) {
		setUnitPrice(observation, "kg")
	})
	history := fixture.history("1", "2026-10-27", "2026-10-28")
	assertPrice(test, history.LatestPriceMinor, 2490)
	if history.PriceBasis == nil || *history.PriceBasis != "kg" {
		test.Fatal("explicit per-kg basis lost")
	}
	fixture.execute(`UPDATE price_observations SET price_basis='l'`)
	fixture.submit(2, func(observation map[string]any) {
		setUnitPrice(observation, "kg")
	})
	history = fixture.history("1", "2026-10-27", "2026-10-28")
	if history.LatestPriceMinor != nil {
		test.Fatal("incompatible bases were combined")
	}
}

func TestUnknownConditionsAreFlaggedAndNeverReplaceKnownTerms(test *testing.T) {
	fixture := newFixture(test)
	fixture.submit(1, func(observation map[string]any) {
		offer := observation["offers"].([]any)[0].(map[string]any)
		offer["conditions"] = map[string]any{"payment_method": "unknown", "installments": nil,
			"membership_required": nil, "membership_program": nil, "coupon_required": nil, "minimum_quantity": nil, "description": nil}
		observation["offers"] = observation["offers"].([]any)[:1]
	})
	history := fixture.history("1", "2026-10-27", "2026-10-27")
	assertPrice(test, history.LatestPriceMinor, 2490)
	if !history.UnknownConditions {
		test.Fatal("unknown conditions were presented as unconditional")
	}
	fixture.submit(2, func(observation map[string]any) { setPrice(observation, "2026-10-28T12:00:00Z", "26.90") })
	history = fixture.history("1", "2026-10-27", "2026-10-28")
	if history.AvailablePeriod == nil || history.AvailablePeriod.Start != "2026-10-28" || history.ChangeBasisPoints != nil {
		test.Fatal("unknown terms must be excluded from the available interval")
	}
}

func TestHistoryShowsAvailableDatesWithinWiderSelection(test *testing.T) {
	fixture := newFixture(test)
	fixture.submit(1, nil)
	fixture.submit(2, func(observation map[string]any) { setPrice(observation, "2026-10-29T12:00:00Z", "26.90") })
	for _, product := range []string{"1", "global"} {
		history := fixture.history(product, "2026-10-01", "2027-01-01")
		assertWiderSelection(test, history)
	}
	empty := fixture.history("1", "2027-01-01", "2027-01-02")
	if empty.AvailablePeriod != nil || empty.ChangeBasisPoints != nil || len(empty.Points) != 0 {
		test.Fatal("an interval outside the data must stay empty")
	}
}

func assertWiderSelection(test *testing.T, history insights.History) {
	test.Helper()
	if history.Period.Start != "2026-10-01" || history.Period.End != "2027-01-01" {
		test.Fatal("requested filters must be preserved")
	}
	if history.AvailablePeriod == nil || history.AvailablePeriod.Start != "2026-10-27" || history.AvailablePeriod.End != "2026-10-29" {
		test.Fatal("available interval must reflect prices inside the request")
	}
	if len(history.Points) != 3 || history.Points[1].Value != nil || history.Counts.PricedDays != 2 {
		test.Fatal("chart must show available days and keep internal gaps")
	}
	assertPrice(test, history.ChangeBasisPoints, 803)
}

func checkMissingCoverage(test *testing.T, history insights.History) {
	test.Helper()
	if history.Points[1].Value != nil {
		test.Fatal("missing prices were filled")
	}
	if history.Counts.Streams != 1 || history.Counts.PricedDays != 2 {
		test.Fatalf("incorrect coverage: %v", history.Counts)
	}
}

func TestUnavailableShopDoesNotBlockMatchedProductOrOverview(test *testing.T) {
	fixture := newFixture(test)
	fixture.submit(1, nil)
	fixture.submit(2, func(observation map[string]any) { setPrice(observation, "2026-10-28T12:00:00Z", "24.90") })
	fixture.submit(3, unavailableListing)
	fixture.execute(`INSERT INTO shops VALUES ('other-shop','Other shop','https://other.example.com')`)
	fixture.execute(`UPDATE shop_products SET shop_id='other-shop' WHERE source_product_id='unavailable-listing'`)
	history := fixture.history("1", "2026-10-27", "2026-10-28")
	assertPrice(test, history.ChangeBasisPoints, 0)
	assertPrice(test, history.LatestPriceMinor, 2490)
	if history.Counts.Shops != 2 || history.Counts.IncludedShops != 1 || len(history.ShopPrices) != 2 {
		test.Fatalf("unexpected shop coverage: %+v", history.Counts)
	}
	global := fixture.history("global", "2026-10-27", "2026-10-28")
	assertPrice(test, global.ChangeBasisPoints, 0)
}

func unavailableListing(observation map[string]any) {
	observation["product"].(map[string]any)["source_product_id"] = "unavailable-listing"
	observation["availability"], observation["offers"] = "out_of_stock", []any{}
}

func TestEquivalenceCombinesHistorySearchAndGlobal(test *testing.T) {
	fixture := newFixture(test)
	fixture.submit(1, nil)
	fixture.submit(2, func(observation map[string]any) { setPrice(observation, "2026-10-28T12:00:00Z", "26.90") })
	fixture.submit(3, secondEquivalentListing)
	fixture.submit(4, func(observation map[string]any) {
		secondEquivalentListing(observation)
		setPrice(observation, "2026-10-28T12:00:00Z", "33.00")
	})
	snapshot, err := equivalence.Read(fixture.ctx, fixture.pool)
	requireSuccess(test, err)
	decision := equivalence.Decision{Revision: snapshot.Revision, Action: "approve", First: 1, Second: 2, Reason: "Verified product and package", SourceVerified: true}
	requireSuccess(test, equivalence.Apply(fixture.ctx, fixture.pool, "owner", decision))
	assertSearch(test, fixture, "selecionado", "1")
	verifyCombinedHistory(test, fixture)
}

func verifyCombinedHistory(test *testing.T, fixture fixture) {
	test.Helper()
	history := fixture.history("2", "2026-10-27", "2026-10-28")
	if history.Counts.Products != 1 || history.Counts.Streams != 2 {
		test.Fatalf("group streams lost: %+v", history.Counts)
	}
	assertPrice(test, history.LatestPriceMinor, 2690)
	assertPrice(test, history.Points[1].Value, 2995)
	global := fixture.history("global", "2026-10-27", "2026-10-28")
	if global.Counts.Products != 1 {
		test.Fatal("global counted members independently")
	}
	fixture.execute(`UPDATE shop_products SET product_id=NULL`)
	assertPrice(test, fixture.history("2", "2026-10-27", "2026-10-28").LatestPriceMinor, 2690)
}

func secondEquivalentListing(observation map[string]any) {
	product := observation["product"].(map[string]any)
	product["source_product_id"] = "other-product"
	product["gtin"] = "7891234567888"
	product["name"] = "Arroz Branco Selecionado Tipo 1 5kg"
	setPrice(observation, "2026-10-27T12:00:00Z", "30.00")
}

func TestGenericFutureObservationsAndReversal(test *testing.T) {
	fixture := newFixture(test)
	snapshot, err := equivalence.Read(fixture.ctx, fixture.pool)
	requireSuccess(test, err)
	requireSuccess(test, equivalence.Apply(fixture.ctx, fixture.pool, "owner", equivalence.Decision{Revision: snapshot.Revision, Action: "activate", Reason: "Reviewed preview"}))
	fixture.submit(1, bananaListing)
	fixture.submit(2, func(observation map[string]any) {
		bananaListing(observation)
		product := observation["product"].(map[string]any)
		product["source_product_id"] = "other-banana"
		product["gtin"] = "7891234567888"
	})
	snapshot, err = equivalence.Read(fixture.ctx, fixture.pool)
	requireSuccess(test, err)
	if len(snapshot.Products) != 2 || snapshot.Products[1].Representative != 1 {
		test.Fatal("future kg observations not grouped", snapshot.Products)
	}
	verifyGenericReversal(test, fixture, snapshot.Revision)
}

func verifyGenericReversal(test *testing.T, fixture fixture, revision int64) {
	test.Helper()
	requireSuccess(test, equivalence.Apply(fixture.ctx, fixture.pool, "owner", equivalence.Decision{Revision: revision, Action: "dissolve", First: 1, Reason: "Source correction"}))
	fixture.submit(3, func(observation map[string]any) {
		bananaListing(observation)
		setPrice(observation, "2026-10-28T12:00:00Z", "6.00")
	})
	snapshot, err := equivalence.Read(fixture.ctx, fixture.pool)
	requireSuccess(test, err)
	if snapshot.Products[1].Representative != 2 {
		test.Fatal("dissolved pair regrouped on ingestion")
	}
}

func bananaListing(observation map[string]any) {
	setUnitPrice(observation, "kg")
	product := observation["product"].(map[string]any)
	product["name"], product["brand"] = "Banana Maçã Kg", "Hortifruti"
}

func newHirotaFixture(test *testing.T) fixture {
	fixture := newFixture(test)
	fixture.execute(`INSERT INTO shops VALUES ('hirota','Hirota','https://www.lojahirota.com.br')`)
	router, err := ingestion.NewRouter(ingestion.PostgresStore{DB: fixture.pool}, ingestion.Auth{
		Token: "test", ScraperID: "collector-sp-01", ShopIDs: []string{"hirota"}})
	requireSuccess(test, err)
	RegisterRoutes(router, PostgresStore{DB: fixture.pool})
	fixture.router = router
	return fixture
}

func TestCapturedHirotaPickupPricesSurviveIngestionAndWebHistory(test *testing.T) {
	fixture := newHirotaFixture(test)
	fixture.submitStoreSource(1, "2026-10-31T12:00:00Z", "hirota-submission")
	var products Products
	fixture.get("/api/v1/products", &products)
	if len(products.Items) != 0 {
		test.Fatal("one local date must not qualify Hirota for search")
	}
	fixture.submitStoreSource(2, "2026-11-01T12:00:00Z", "hirota-submission")
	fixture.get("/api/v1/products?q=Arroz", &products)
	if len(products.Items) != 1 {
		test.Fatalf("Hirota rice was not searchable: %+v", products)
	}
	assertHirotaRiceHistory(fixture, products.Items[0])
	assertHirotaWeightPersistence(fixture)
	assertHirotaWeightHistory(fixture)
}

func assertHirotaRiceHistory(fixture fixture, product Product) {
	history := fixture.history(product.ID, "2026-10-31", "2026-11-01")
	assertPrice(fixture.test, history.LatestPriceMinor, 2898)
	assertPrice(fixture.test, history.ChangeBasisPoints, 0)
	if !strings.Contains(history.Coverage, "São Paulo/SP") || len(history.ShopPrices) != 1 {
		fixture.test.Fatalf("Hirota geography or shop missing: %+v", history)
	}
	shop := history.ShopPrices[0]
	if shop.ShopID != "hirota" || shop.ShopName != "Hirota" || !strings.Contains(shop.Context, "04263100") {
		fixture.test.Fatalf("Hirota pickup context lost: %+v", shop)
	}
	if shop.ProductURL != "https://www.lojahirota.com.br/produto/498/arroz-camil-tipo-1-5kg" {
		fixture.test.Fatalf("Hirota public product link lost: %s", shop.ProductURL)
	}
}

func assertHirotaWeightPersistence(fixture fixture) {
	pool := fixture.pool
	row := pool.QueryRowContext(fixture.ctx, `SELECT count(*)=4 AND bool_and(
		package_quantity IS NULL AND price_basis='kg' AND postal_code='04263100'
		AND city='São Paulo' AND state='SP' AND channel='pickup' AND seller_id IS NULL
		AND store_id='org:171/filial:1/centro:1' AND price_amount IN (19.00,7.98))
		FROM price_observations WHERE price_basis='kg'`)
	var preserved bool
	requireSuccess(fixture.test, row.Scan(&preserved))
	if !preserved {
		fixture.test.Fatal("Hirota kg prices or pickup provenance lost during processing")
	}
}

func assertHirotaWeightHistory(fixture fixture) {
	var products Products
	fixture.get("/api/v1/products?q=Tomate", &products)
	if len(products.Items) != 1 {
		fixture.test.Fatalf("Hirota tomato was not searchable: %+v", products)
	}
	history := fixture.history(products.Items[0].ID, "2026-10-31", "2026-11-01")
	assertPrice(fixture.test, history.LatestPriceMinor, 1900)
	assertPrice(fixture.test, history.ChangeBasisPoints, 0)
	if history.PriceBasis == nil || *history.PriceBasis != "kg" || !history.UnknownConditions {
		fixture.test.Fatalf("Hirota price basis or unknown terms lost: %+v", history)
	}
}

func newPaoDeAcucarFixture(test *testing.T) fixture {
	fixture := newFixture(test)
	fixture.execute(`INSERT INTO shops VALUES
		('pao-de-acucar','Pão de Açúcar','https://www.paodeacucar.com')`)
	router, err := ingestion.NewRouter(ingestion.PostgresStore{DB: fixture.pool}, ingestion.Auth{
		Token: "test", ScraperID: "collector-sp-01", ShopIDs: []string{"pao-de-acucar"}})
	requireSuccess(test, err)
	RegisterRoutes(router, PostgresStore{DB: fixture.pool})
	fixture.router = router
	return fixture
}

func TestCapturedPaoDeAcucarIngestionAndHistory(test *testing.T) {
	fixture := newPaoDeAcucarFixture(test)
	fixture.submitStoreSource(1, "2026-10-31T12:00:00Z", "pao-de-acucar-submission")
	var products Products
	fixture.get("/api/v1/products", &products)
	if len(products.Items) != 0 {
		test.Fatal("one local date must not qualify for search")
	}
	fixture.submitStoreSource(2, "2026-11-01T12:00:00Z", "pao-de-acucar-submission")
	fixture.get("/api/v1/products?q=Arroz", &products)
	if len(products.Items) != 1 {
		test.Fatalf("Pão de Açúcar rice was not searchable: %+v", products)
	}
	assertPaoDeAcucarHistory(fixture, products.Items[0])
	assertPaoDeAcucarWeight(fixture)
}

func assertPaoDeAcucarHistory(fixture fixture, product Product) {
	history := fixture.history(product.ID, "2026-10-31", "2026-11-01")
	assertPrice(fixture.test, history.LatestPriceMinor, 529)
	assertPrice(fixture.test, history.ChangeBasisPoints, 0)
	if len(history.ShopPrices) != 1 || !history.UnknownConditions {
		fixture.test.Fatalf("retailer or unknown conditions lost: %+v", history)
	}
	shop := history.ShopPrices[0]
	if shop.ShopName != "Pão de Açúcar" || !strings.Contains(shop.Context, "04115090") {
		fixture.test.Fatalf("São Paulo retailer context lost: %+v", shop)
	}
	if !strings.Contains(shop.ProductURL, "paodeacucar.com/produto/172/") {
		fixture.test.Fatalf("product link lost: %+v", shop)
	}
}

func assertPaoDeAcucarWeight(fixture fixture) {
	pool := fixture.pool
	row := pool.QueryRowContext(fixture.ctx, `SELECT count(*)=2 AND bool_and(
		package_quantity IS NULL AND price_basis='kg' AND price_amount=12.99
		AND city='São Paulo' AND state='SP' AND postal_code='04115090'
		AND store_id='461' AND channel='delivery' AND seller_id IS NULL)
		FROM price_observations WHERE price_basis='kg'`)
	var preserved bool
	requireSuccess(fixture.test, row.Scan(&preserved))
	if !preserved {
		fixture.test.Fatal("GPA advertised kg price or provenance lost")
	}
}

func newSondaFixture(test *testing.T) fixture {
	fixture := newFixture(test)
	fixture.execute(`INSERT INTO shops VALUES ('sonda','Sonda','https://www.sondadelivery.com.br')`)
	router, err := ingestion.NewRouter(ingestion.PostgresStore{DB: fixture.pool}, ingestion.Auth{
		Token: "test", ScraperID: "collector-sp-01", ShopIDs: []string{"sonda"}})
	requireSuccess(test, err)
	RegisterRoutes(router, PostgresStore{DB: fixture.pool})
	fixture.router = router
	return fixture
}

func TestCapturedSondaIngestionAndHistory(test *testing.T) {
	fixture := newSondaFixture(test)
	fixture.submitStoreSource(1, "2026-10-31T12:00:00Z", "sonda-submission")
	var products Products
	fixture.get("/api/v1/products?q=Camil", &products)
	if len(products.Items) != 0 {
		test.Fatal("one local date must not qualify for search")
	}
	fixture.submitStoreSource(2, "2026-11-01T12:00:00Z", "sonda-submission")
	fixture.get("/api/v1/products?q=Camil", &products)
	if len(products.Items) != 1 {
		test.Fatalf("Sonda rice was not searchable: %+v", products)
	}
	assertSondaHistory(fixture, products.Items[0])
	assertSondaUnknownContext(fixture)
}

func assertSondaHistory(fixture fixture, product Product) {
	history := fixture.history(product.ID, "2026-10-31", "2026-11-01")
	assertPrice(fixture.test, history.LatestPriceMinor, 692)
	assertPrice(fixture.test, history.ChangeBasisPoints, 0)
	if len(history.ShopPrices) != 1 || !history.UnknownConditions {
		fixture.test.Fatalf("Sonda retailer or unknown conditions lost: %+v", history)
	}
	shop := history.ShopPrices[0]
	if shop.ShopID != "sonda" || shop.ShopName != "Sonda" {
		fixture.test.Fatalf("Sonda identity lost: %+v", shop)
	}
	if !strings.HasPrefix(shop.ProductURL, "https://www.sondadelivery.com.br/delivery/produto/") {
		fixture.test.Fatalf("Sonda product link lost: %s", shop.ProductURL)
	}
}

func assertSondaUnknownContext(fixture fixture) {
	pool := fixture.pool
	row := pool.QueryRowContext(fixture.ctx, `SELECT count(*)=2 AND bool_and(
		city IS NULL AND state IS NULL AND postal_code IS NULL AND store_id IS NULL
		AND seller_id IS NULL AND channel='online' AND price_amount=6.92
		AND price_basis='package' AND package_quantity=1 AND package_unit='kg')
		FROM price_observations`)
	var preserved bool
	requireSuccess(fixture.test, row.Scan(&preserved))
	if !preserved {
		fixture.test.Fatal("Sonda price or explicit unknown fulfillment provenance lost")
	}
}

func newNagumoFixture(test *testing.T) fixture {
	fixture := newFixture(test)
	fixture.execute(`INSERT INTO shops VALUES ('nagumo','Nagumo','https://www.nagumo.com.br')`)
	router, err := ingestion.NewRouter(ingestion.PostgresStore{DB: fixture.pool}, ingestion.Auth{
		Token: "test", ScraperID: "collector-sp-01", ShopIDs: []string{"nagumo"}})
	requireSuccess(test, err)
	RegisterRoutes(router, PostgresStore{DB: fixture.pool})
	fixture.router = router
	return fixture
}

func TestCapturedNagumoPickupSurvivesIngestionAndWebHistory(test *testing.T) {
	fixture := newNagumoFixture(test)
	fixture.submitStoreSource(1, "2026-10-31T12:00:00Z", "nagumo-submission")
	var products Products
	fixture.get("/api/v1/products", &products)
	if len(products.Items) != 0 {
		test.Fatal("one local date must not qualify Nagumo for search")
	}
	fixture.submitStoreSource(2, "2026-11-01T12:00:00Z", "nagumo-submission")
	assertNagumoHistory(fixture, "Arroz", 2370, "package")
	assertNagumoHistory(fixture, "Banana Nanica", 698, "kg")
	assertNagumoProvenance(fixture)
}

func assertNagumoHistory(fixture fixture, query string, amount int64, basis string) {
	var products Products
	fixture.get("/api/v1/products?q="+strings.ReplaceAll(query, " ", "%20"), &products)
	if len(products.Items) != 1 {
		fixture.test.Fatalf("Nagumo product was not searchable: %+v", products)
	}
	history := fixture.history(products.Items[0].ID, "2026-10-31", "2026-11-01")
	assertPrice(fixture.test, history.LatestPriceMinor, amount)
	assertPrice(fixture.test, history.ChangeBasisPoints, 0)
	if history.PriceBasis == nil || *history.PriceBasis != basis || !history.UnknownConditions {
		fixture.test.Fatalf("Nagumo basis or unknown conditions lost: %+v", history)
	}
	assertNagumoPresentation(fixture, history)
}

func assertNagumoPresentation(fixture fixture, history insights.History) {
	if !strings.Contains(history.Coverage, "São Paulo/SP") || len(history.ShopPrices) != 1 {
		fixture.test.Fatalf("Nagumo geography or retailer lost: %+v", history)
	}
	shop := history.ShopPrices[0]
	if shop.ShopID != "nagumo" || shop.ShopName != "Nagumo" || !strings.Contains(shop.Context, "08030040") {
		fixture.test.Fatalf("Nagumo pickup context lost: %+v", shop)
	}
	if !strings.HasPrefix(shop.ProductURL, "https://www.nagumo.com.br/categoria/") {
		fixture.test.Fatalf("Nagumo public product link lost: %s", shop.ProductURL)
	}
}

func assertNagumoProvenance(fixture fixture) {
	pool := fixture.pool
	row := pool.QueryRowContext(fixture.ctx, `SELECT count(*)=4 AND bool_and(
		postal_code='08030040' AND city='São Paulo' AND state='SP' AND channel='pickup'
		AND store_id='13' AND seller_id IS NULL AND package_quantity IS NULL AND price_basis='kg'
		AND identity.collector_name='nagumo' AND source_url LIKE 'https://www.nagumo.com.br/on/demandware.store/%'
		AND ((price_amount=6.98 AND membership_required IS NULL)
		OR (price_amount=2.98 AND membership_required AND membership_program='Meu Nagumo')))
		FROM price_observations observation JOIN shop_products listing ON listing.id=shop_product_id
		JOIN observation_identities identity USING (scraper_id, observation_id)
		WHERE listing.source_product_id='2004'`)
	var preserved bool
	requireSuccess(fixture.test, row.Scan(&preserved))
	if !preserved {
		fixture.test.Fatal("Nagumo kg prices, membership conditions or pickup provenance lost")
	}
}
