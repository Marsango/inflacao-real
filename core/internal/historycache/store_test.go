package historycache

import (
	"context"
	"errors"
	"testing"
	"time"

	"inflacao-real/core/internal/insights"
	"inflacao-real/core/internal/webapi"
)

func TestHitsReturnIndependentHistories(test *testing.T) {
	source := &stubSource{}
	store := newTestStore(test, source, DefaultConfig())
	first := readHistory(test, store, defaultRequest())
	first.Points[0].Value = nil
	first.ShopPrices[0].Points[0].Value = nil
	*first.CollectedAt = time.Time{}
	second := readHistory(test, store, defaultRequest())
	if second.Points[0].Value == nil || second.ShopPrices[0].Points[0].Value == nil || second.CollectedAt.IsZero() {
		test.Fatal("caller modified the cached history")
	}
	assertCalls(test, source, 1)
	if statistics := store.Stats(); statistics.Hits != 1 || statistics.Misses != 1 || statistics.Calculations != 1 {
		test.Fatalf("unexpected counters: %+v", statistics)
	}
}

func TestProductAndBothDatesSeparateEntries(test *testing.T) {
	source := &stubSource{}
	store := newTestStore(test, source, DefaultConfig())
	request := defaultRequest()
	readHistory(test, store, request)
	request.Product = "1"
	readHistory(test, store, request)
	request.Start = "2026-10-02"
	readHistory(test, store, request)
	request.End = "2026-10-31"
	readHistory(test, store, request)
	readHistory(test, store, request)
	assertCalls(test, source, 4)
}

func TestExpiryDoesNotExtendOnHits(test *testing.T) {
	source := &stubSource{}
	store := newTestStore(test, source, DefaultConfig())
	now := time.Now()
	store.now = func() time.Time { return now }
	readHistory(test, store, defaultRequest())
	now = now.Add(4 * time.Minute)
	readHistory(test, store, defaultRequest())
	assertCalls(test, source, 1)
	now = now.Add(time.Minute)
	readHistory(test, store, defaultRequest())
	assertCalls(test, source, 2)
}

func TestFreshnessCrosses48HoursWithoutRecalculation(test *testing.T) {
	source := &stubSource{}
	store := newTestStore(test, source, DefaultConfig())
	collected := *sampleHistory("").CollectedAt
	now := collected.Add(48 * time.Hour)
	store.now = func() time.Time { return now }
	first := readHistory(test, store, defaultRequest())
	now = now.Add(time.Nanosecond)
	second := readHistory(test, store, defaultRequest())
	if first.Stale || !second.Stale || !second.CollectedAt.Equal(collected) {
		test.Fatal("freshness or source timestamp changed incorrectly")
	}
	assertCalls(test, source, 1)
}

func TestSuccessfulEmptyResultsAreCached(test *testing.T) {
	source := &stubSource{history: func(context.Context, webapi.HistoryRequest) (insights.History, error) {
		return insights.Build(nil, true, 0, time.Now()), nil
	}}
	store := newTestStore(test, source, DefaultConfig())
	readHistory(test, store, defaultRequest())
	result := readHistory(test, store, defaultRequest())
	if result.Stale || len(result.Points) != 0 || result.CollectedAt != nil {
		test.Fatal("empty results must preserve missing values")
	}
	assertCalls(test, source, 1)
}

func TestFailuresAreRetried(test *testing.T) {
	for _, failure := range []error{webapi.ErrNotFound, errors.New("database unavailable")} {
		assertFailureRetried(test, failure)
	}
}

func assertFailureRetried(test *testing.T, failure error) {
	test.Helper()
	source := &stubSource{history: func(context.Context, webapi.HistoryRequest) (insights.History, error) {
		return insights.History{}, failure
	}}
	store := newTestStore(test, source, DefaultConfig())
	_, first := store.History(test.Context(), defaultRequest())
	_, second := store.History(test.Context(), defaultRequest())
	if !errors.Is(first, failure) || !errors.Is(second, failure) || store.Stats().Entries != 0 {
		test.Fatal("failure changed or was cached")
	}
	assertCalls(test, source, 2)
}

func TestRevisionFailureDoesNotServeCachedData(test *testing.T) {
	source := &stubSource{}
	store := newTestStore(test, source, DefaultConfig())
	readHistory(test, store, defaultRequest())
	failure := errors.New("revision unavailable")
	source.revisionFailure = failure
	_, err := store.History(test.Context(), defaultRequest())
	if !errors.Is(err, failure) {
		test.Fatalf("expected revision error, got %v", err)
	}
	assertCalls(test, source, 1)
}

func TestProductsPassThrough(test *testing.T) {
	store := newTestStore(test, &stubSource{}, DefaultConfig())
	products, err := store.Products(test.Context(), webapi.ProductRequest{Query: "arroz", Limit: 20})
	requireSuccess(test, err)
	if len(products.Items) != 1 || products.Items[0].Label != "Arroz" {
		test.Fatalf("catalog pass-through failed: %+v", products)
	}
}

func TestCuratedGroupsHaveSeparateCachedHistories(test *testing.T) {
	source := &stubSource{}
	store := newTestStore(test, source, DefaultConfig())
	request := defaultRequest()
	readHistory(test, store, request)
	request.Group = "cesta-basica"
	readHistory(test, store, request)
	request.Group = "hortifruti"
	readHistory(test, store, request)
	request.Group = "cesta-basica"
	readHistory(test, store, request)
	assertCalls(test, source, 3)
}
