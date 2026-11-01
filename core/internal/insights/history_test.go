package insights

import (
	"strings"
	"testing"
	"time"
)

func TestOverviewDisclosesCitiesWithoutComparingDifferentLocations(test *testing.T) {
	curitiba := testContext("shop", "1000", "")
	curitiba.Location = "Curitiba/PR"
	saoPaulo := testContext("shop", "", "2000")
	saoPaulo.Location = "São Paulo/SP"
	product := ProductSeries{Contexts: []ContextSeries{curitiba, saoPaulo, curitiba}, Streams: 3, Shops: 1}
	history := Build([]ProductSeries{product}, true, 1, time.Now())
	if history.ChangeBasisPoints != nil || history.Counts.IncludedStreams != 0 {
		test.Fatal("different locations must not create a temporal price change")
	}
	if !strings.Contains(history.Coverage, "Localidades observadas: Curitiba/PR; São Paulo/SP.") {
		test.Fatalf("missing or duplicated geography: %s", history.Coverage)
	}
}

func testSeries(name string, values ...string) ProductSeries {
	context := testContext("shop", values...)
	return ProductSeries{Name: name, Streams: 1, Shops: 1, Contexts: []ContextSeries{context}}
}

func testContext(shop string, values ...string) ContextSeries {
	context := ContextSeries{ShopID: shop, ShopName: shop}
	for position, value := range values {
		context.Points = append(context.Points, testPrice(position, value))
	}
	return context
}

func testPrice(position int, value string) PricePoint {
	point := PricePoint{Date: time.Date(2026, 10, 27+position, 0, 0, 0, 0, time.UTC).Format("2006-01-02")}
	if value != "" {
		point.Amount = &value
	}
	return point
}

func assertValue(test *testing.T, actual *int64, expected int64) {
	test.Helper()
	if actual == nil || *actual != expected {
		test.Fatalf("got %v, expected %d", actual, expected)
	}
}

func TestMatchedChangesKeepGapsAndExactMeans(test *testing.T) {
	product := testSeries("Arroz", "2001/2", "", "2201/2")
	history := Build([]ProductSeries{product}, false, 1, time.Now())
	assertValue(test, history.LatestPriceMinor, 1101)
	assertValue(test, history.ChangeBasisPoints, 1000)
	if history.Points[1].Value != nil || history.Counts.PricedDays != 2 {
		test.Fatal("missing day must remain missing")
	}
}

func TestShopsHaveEqualWeightRegardlessOfContextCount(test *testing.T) {
	product := testSeries("Café", "2000", "2200")
	product.Contexts = append(product.Contexts, testContext("shop", "1000", "1100"), testContext("other", "2500", "2500"))
	product.Contexts = append(product.Contexts, testContext("unavailable", "1800", ""))
	product.Streams, product.Shops = 4, 3
	history := Build([]ProductSeries{product}, false, 3, time.Now())
	assertValue(test, history.ChangeBasisPoints, 500)
	assertValue(test, history.Points[0].Value, 2000)
	assertValue(test, history.Points[1].Value, 2075)
	if history.Counts.IncludedStreams != 3 || history.Counts.IncludedShops != 2 || history.ShopPrices[3].Included {
		test.Fatal("incorrect comparison coverage")
	}
}

func TestUnavailableShopDoesNotHidePricesOrChangeVariation(test *testing.T) {
	product := testSeries("Café", "2780", "2780")
	product.Contexts = append(product.Contexts, testContext("unavailable", "", ""))
	product.Streams, product.Shops = 2, 2
	history := Build([]ProductSeries{product}, false, 2, time.Now())
	assertValue(test, history.LatestPriceMinor, 2780)
	assertValue(test, history.ChangeBasisPoints, 0)
	if len(history.ShopPrices) != 2 || history.ShopPrices[1].Points[0].Value != nil {
		test.Fatal("shop prices lost or fabricated")
	}
}

func TestOverviewExcludesUnmatchedProductsAndWeightsProductsEqually(test *testing.T) {
	products := []ProductSeries{testSeries("Arroz", "1000", "1100", "1200"), testSeries("Café", "10000", "", "9000")}
	products = append(products, testSeries("Unavailable", "100", "100", ""))
	history := Build(products, true, 1, time.Now())
	assertValue(test, history.Points[0].Value, 0)
	assertValue(test, history.ChangeBasisPoints, 500)
	if history.Points[1].Value != nil || history.Counts.IncludedProducts != 2 || history.Counts.Products != 3 {
		test.Fatal("incomplete panel or exclusions incorrect")
	}
}

func TestNoShopSwitchOrMovingBaseline(test *testing.T) {
	product := testSeries("Café", "1000", "")
	product.Contexts = append(product.Contexts, testContext("other", "", "2000"))
	history := Build([]ProductSeries{product}, false, 2, time.Now())
	assertValue(test, history.LatestPriceMinor, 2000)
	if history.ChangeBasisPoints != nil || history.ComparisonStart != nil || history.Counts.IncludedProducts != 0 {
		test.Fatal("availability switch invented a price change")
	}
	zero := Build([]ProductSeries{testSeries("Zero", "0", "100")}, true, 1, time.Now())
	if zero.ChangeBasisPoints != nil || zero.Counts.IncludedProducts != 0 {
		test.Fatal("zero baseline was included")
	}
}

func TestSingleDayAndEmptyHistory(test *testing.T) {
	single := Build([]ProductSeries{testSeries("Leite", "100")}, false, 1, time.Now())
	assertValue(test, single.LatestPriceMinor, 100)
	if single.ChangeBasisPoints != nil || single.ComparisonStart != nil {
		test.Fatal("single day invented variation")
	}
	empty := Build(nil, true, 0, time.Now())
	if empty.Points == nil || empty.ShopPrices == nil || empty.CollectedAt != nil {
		test.Fatal("empty history must preserve unknowns and arrays")
	}
}

func TestFreshnessUsesObservationTime(test *testing.T) {
	collected := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	product := testSeries("Arroz", "100")
	product.CollectedAt = &collected
	fresh := Build([]ProductSeries{product}, false, 1, collected.Add(48*time.Hour))
	stale := Build([]ProductSeries{product}, false, 1, collected.Add(48*time.Hour+time.Second))
	if fresh.Stale || !stale.Stale {
		test.Fatal("freshness boundary must be 48 hours")
	}
}

func TestRoundingAndSafeJSONIntegers(test *testing.T) {
	negative := "-1/2"
	assertValue(test, rounded(amount(&negative)), -1)
	huge := "10000000000000000"
	if rounded(amount(&huge)) != nil {
		test.Fatal("unsafe JSON integer must remain unavailable")
	}
}

func TestAveragesContextRelativesRatherThanPriceLevels(test *testing.T) {
	product := testSeries("Café", "1000", "2000")
	product.Contexts = append(product.Contexts, testContext("shop", "10000", "10000"))
	history := Build([]ProductSeries{product}, false, 1, time.Now())
	assertValue(test, history.ChangeBasisPoints, 5000)
	assertValue(test, history.Points[0].Value, 5500)
	assertValue(test, history.Points[1].Value, 6000)
}

func TestZeroEndpointRemainsARealPrice(test *testing.T) {
	history := Build([]ProductSeries{testSeries("Café", "1000", "0")}, false, 1, time.Now())
	assertValue(test, history.LatestPriceMinor, 0)
	assertValue(test, history.ChangeBasisPoints, -10000)
}

func TestAvailableIntervalIgnoresEmptySelectedEndpoints(test *testing.T) {
	series := []ProductSeries{testSeries("Café", "", "1000", "", "1100", "")}
	for _, global := range []bool{false, true} {
		history := Build(series, global, 1, time.Now())
		assertAvailableInterval(test, history)
	}
}

func assertAvailableInterval(test *testing.T, history History) {
	test.Helper()
	expected := Period{Start: "2026-10-28", End: "2026-10-30"}
	if history.AvailablePeriod == nil || *history.AvailablePeriod != expected {
		test.Fatalf("incorrect available dates: %+v", history.AvailablePeriod)
	}
	if *history.ComparisonStart != expected.Start || *history.ComparisonEnd != expected.End {
		test.Fatal("comparison must disclose the actual dates used")
	}
	assertValue(test, history.ChangeBasisPoints, 1000)
	if len(history.Points) != 3 || history.Points[1].Value != nil || history.Counts.PricedDays != 2 {
		test.Fatal("trimmed chart must preserve internal gaps")
	}
}

func TestSingleAvailableDayHasPricesWithoutChange(test *testing.T) {
	series := []ProductSeries{testSeries("Leite", "", "100", "")}
	for _, global := range []bool{false, true} {
		history := Build(series, global, 1, time.Now())
		assertSingleAvailableDay(test, history, global)
	}
}

func assertSingleAvailableDay(test *testing.T, history History, global bool) {
	test.Helper()
	if len(history.Points) != 1 || history.Counts.PricedDays != 1 {
		test.Fatal("one available day must remain visible")
	}
	if history.ChangeBasisPoints != nil || history.ComparisonStart != nil || history.ComparisonEnd != nil {
		test.Fatal("one day cannot provide a change")
	}
	if global {
		assertValue(test, history.Points[0].Value, 0)
		return
	}
	assertValue(test, history.Points[0].Value, 100)
}

func TestAvailablePricesDoNotInventComparableVariation(test *testing.T) {
	product := testSeries("Café", "", "1000", "", "", "")
	product.Contexts = append(product.Contexts, testContext("other", "", "", "", "2000", ""))
	history := Build([]ProductSeries{product}, false, 2, time.Now())
	assertValue(test, history.Points[0].Value, 1000)
	assertValue(test, history.Points[2].Value, 2000)
	if history.ChangeBasisPoints != nil || history.Counts.IncludedStreams != 0 || history.Points[1].Value != nil {
		test.Fatal("switching shops cannot produce comparable variation or fill gaps")
	}
}

func TestZeroBaselinePricesRemainVisibleWithoutVariation(test *testing.T) {
	history := Build([]ProductSeries{testSeries("Café", "", "0", "100", "")}, false, 1, time.Now())
	assertValue(test, history.Points[0].Value, 0)
	assertValue(test, history.Points[1].Value, 100)
	if history.ChangeBasisPoints != nil || history.ComparisonStart != nil {
		test.Fatal("zero baseline cannot produce a relative change")
	}
}

func TestNoEligiblePricesHasNoAvailableInterval(test *testing.T) {
	history := Build([]ProductSeries{testSeries("Café", "", "", "")}, false, 1, time.Now())
	if history.AvailablePeriod != nil || history.ChangeBasisPoints != nil || history.Counts.PricedDays != 0 {
		test.Fatal("no eligible prices must not invent dates or values")
	}
}
