package webapi

import (
	"testing"

	"inflacao-real/core/internal/insights"
)

func TestIngestionToPublicHistory(test *testing.T) {
	fixture := newFixture(test)
	fixture.submit(1, nil)
	fixture.submit(2, func(observation map[string]any) { setPrice(observation, "2026-10-29T12:00:00Z", "26.90") })
	var products Products
	fixture.get("/api/v1/products", &products)
	if len(products.Items) != 1 || products.NextCursor != nil {
		test.Fatalf("unexpected products: %v", products)
	}
	history := fixture.history(products.Items[0].ID, "2026-10-26", "2026-10-30")
	assertPrice(test, history.LatestPriceMinor, 2690)
	assertPrice(test, history.ChangeBasisPoints, 803)
	assertPrice(test, history.Points[1].Value, 2490)
	checkMissingCoverage(test, history)
	global := fixture.history("global", "2026-10-26", "2026-10-30")
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
	if history.Points[0].Value != nil || history.Counts.Streams != 2 {
		test.Fatal("panel changed between days")
	}
	assertPrice(test, history.Points[1].Value, 2690)
	if history.ChangeBasisPoints != nil {
		test.Fatal("only one complete day cannot define change")
	}
}

func TestPaginationAndHistoricalProductAssignment(test *testing.T) {
	fixture := newFixture(test)
	fixture.submit(1, nil)
	fixture.execute(`INSERT INTO products (name) VALUES ('Another product'), ('Unobserved product')`)
	fixture.execute(`UPDATE shop_products SET product_id=2`)
	fixture.submit(2, func(observation map[string]any) {
		setPrice(observation, "2026-10-28T12:00:00Z", "10.00")
		product := observation["product"].(map[string]any)
		product["name"], product["brand"], product["gtin"], product["package"] = "Another product", nil, nil, nil
	})
	history := fixture.history("1", "2026-10-27", "2026-10-27")
	assertPrice(test, history.LatestPriceMinor, 2490)
	checkCatalogPages(test, fixture)

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
	if history.Points[0].Value != nil || history.ChangeBasisPoints != nil {
		test.Fatal("unknown and known terms were mixed")
	}
}

func checkMissingCoverage(test *testing.T, history insights.History) {
	test.Helper()
	if history.Points[2].Value != nil || history.Points[4].Value != nil {
		test.Fatal("missing prices were filled")
	}
	if history.Counts.Streams != 1 || history.Counts.PricedDays != 2 {
		test.Fatalf("incorrect coverage: %v", history.Counts)
	}
}
