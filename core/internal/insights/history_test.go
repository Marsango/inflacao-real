package insights

import (
	"testing"
	"time"
)

func testSeries(name string, values ...string) ProductSeries {
	product := ProductSeries{Name: name, Streams: 1, Shops: 1}
	for position, value := range values {
		product.Points = append(product.Points, testPrice(position, value))
	}
	return product
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

func TestProductChangeUsesUnroundedMeansAndKeepsGaps(test *testing.T) {
	series := testSeries("Arroz", "2001/2", "", "2201/2")
	history := Build([]ProductSeries{series}, false, 1, time.Now())
	assertValue(test, history.LatestPriceMinor, 1101)
	assertValue(test, history.ChangeBasisPoints, 1000)
	if history.Points[1].Value != nil || history.Counts.PricedDays != 2 {
		test.Fatal("missing day must remain missing")
	}
	if *history.ComparisonStart != "2026-10-27" || *history.ComparisonEnd != "2026-10-29" {
		test.Fatal("comparison dates lost")
	}
}

func TestOverviewWeightsProductsEquallyAndRequiresCompletePanel(test *testing.T) {
	products := []ProductSeries{testSeries("Arroz", "1000", "1100", "1200"), testSeries("Café", "10000", "", "9000")}
	history := Build(products, true, 1, time.Now())
	assertValue(test, history.Points[0].Value, 0)
	assertValue(test, history.ChangeBasisPoints, 500)
	if history.Points[1].Value != nil || history.LatestPriceMinor != nil {
		test.Fatal("incomplete panel is not a price")
	}
	if history.Counts.Products != 2 || history.Counts.Shops != 1 {
		test.Fatal("coverage lost")
	}
}

func TestOverviewWaitsForPositiveCommonBaseline(test *testing.T) {
	products := []ProductSeries{testSeries("Arroz", "0", "100", "200"), testSeries("Café", "100", "200", "200")}
	history := Build(products, true, 1, time.Now())
	if history.Points[0].Value != nil {
		test.Fatal("zero baseline cannot define a relative index")
	}
	assertValue(test, history.Points[1].Value, 0)
	assertValue(test, history.ChangeBasisPoints, 5000)
	if *history.ComparisonStart != "2026-10-28" {
		test.Fatal("wrong index baseline")
	}
}

func TestSinglePriceZeroBaselineAndNoCoverage(test *testing.T) {
	single := Build([]ProductSeries{testSeries("Leite", "100")}, false, 1, time.Now())
	if single.ChangeBasisPoints != nil {
		test.Fatal("one measurement cannot define change")
	}
	zero := Build([]ProductSeries{testSeries("Leite", "0", "100")}, false, 1, time.Now())
	if zero.ChangeBasisPoints != nil {
		test.Fatal("zero baseline cannot define change")
	}
	assertValue(test, zero.Points[0].Value, 0)
	empty := Build(nil, true, 0, time.Now())
	if empty.Points == nil || empty.CollectedAt != nil || empty.ChangeBasisPoints != nil {
		test.Fatal("empty history must preserve unknowns")
	}
}

func TestFreshnessUsesObservationTimeAndNotRequestedEnd(test *testing.T) {
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
