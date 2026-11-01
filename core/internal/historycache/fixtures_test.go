package historycache

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"inflacao-real/core/internal/insights"
	"inflacao-real/core/internal/webapi"
)

type stubSource struct {
	revision        atomic.Int64
	calls           atomic.Int64
	revisionFailure error
	history         func(context.Context, webapi.HistoryRequest) (insights.History, error)
}

func (source *stubSource) Revision(context.Context) (int64, error) {
	return source.revision.Load(), source.revisionFailure
}

func (source *stubSource) Products(context.Context, webapi.ProductRequest) (webapi.Products, error) {
	return webapi.Products{Items: []webapi.Product{{ID: "1", Label: "Arroz"}}}, nil
}

func (source *stubSource) History(requestContext context.Context, request webapi.HistoryRequest) (insights.History, error) {
	source.calls.Add(1)
	if source.history != nil {
		return source.history(requestContext, request)
	}
	return sampleHistory(request.Product), nil
}

func sampleHistory(title string) insights.History {
	value := int64(2490)
	collected := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	return insights.History{Title: title, CollectedAt: &collected, MethodologyVersion: insights.MethodologyVersion,
		Points:     []insights.Point{{Date: "2026-11-01", Value: &value}},
		ShopPrices: []insights.ShopHistory{{ShopName: "Mercado", Points: []insights.Point{{Value: &value}}}}}
}

func defaultRequest() webapi.HistoryRequest {
	return webapi.HistoryRequest{Product: "global", Start: "2026-10-01", End: "2026-11-01"}
}

func newTestStore(test *testing.T, source source, configuration Config) *Store {
	test.Helper()
	store, err := New(test.Context(), source, configuration)
	requireSuccess(test, err)
	return store
}

func requireSuccess(test *testing.T, err error) {
	test.Helper()
	if err != nil {
		test.Fatal(err)
	}
}

func readHistory(test *testing.T, store *Store, request webapi.HistoryRequest) insights.History {
	test.Helper()
	history, err := store.History(test.Context(), request)
	requireSuccess(test, err)
	return history
}

func assertCalls(test *testing.T, source *stubSource, expected int64) {
	test.Helper()
	if calls := source.calls.Load(); calls != expected {
		test.Fatalf("expected %d calculations, got %d", expected, calls)
	}
}

type historyResult struct {
	history insights.History
	err     error
}

func startHistory(store *Store, requestContext context.Context) <-chan historyResult {
	result := make(chan historyResult, 1)
	go func() {
		history, err := store.History(requestContext, defaultRequest())
		result <- historyResult{history, err}
	}()
	return result
}

func receiveHistory(test *testing.T, result <-chan historyResult) historyResult {
	test.Helper()
	select {
	case history := <-result:
		return history
	case <-time.After(5 * time.Second):
		test.Fatal("history did not complete")
		return historyResult{}
	}
}

func awaitSignal(test *testing.T, signal <-chan struct{}) {
	test.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		test.Fatal("calculation did not reach expected stage")
	}
}

func requestForProduct(number int) webapi.HistoryRequest {
	request := defaultRequest()
	request.Product = fmt.Sprint(number)
	return request
}
