package webapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"inflacao-real/core/internal/insights"
)

type fakeStore struct {
	failure error
	request HistoryRequest
	after   int64
	limit   int
}

func (store *fakeStore) Products(ctx context.Context, after int64, limit int) (Products, error) {
	store.after, store.limit = after, limit
	return Products{Items: []Product{}}, store.failure
}

func (store *fakeStore) History(ctx context.Context, request HistoryRequest) (insights.History, error) {
	store.request = request
	return insights.Build(nil, true, 0, timeForTest()), store.failure
}

func getResponse(store Store, path string) *httptest.ResponseRecorder {
	router := gin.New()
	RegisterRoutes(router, store)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
	return response
}

func TestPublicReadsAndFilterForwarding(test *testing.T) {
	store := &fakeStore{}
	response := getResponse(store, "/api/v1/history?product=123&start=2024-02-29&end=2025-02-28")
	if response.Code != 200 || store.request.Product != "123" || store.request.Start != "2024-02-29" {
		test.Fatalf("request not forwarded: %d %v", response.Code, store.request)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		test.Fatal("history must not be cached")
	}
	response = getResponse(store, "/api/v1/products?after=123&limit=20")
	if response.Code != 200 || store.after != 123 || store.limit != 20 {
		test.Fatal("pagination not forwarded")
	}
}

func TestInvalidHistoryFilters(test *testing.T) {
	queries := []string{"", "product=rice&start=2026-10-01&end=2026-10-02", "product=0&start=2026-10-01&end=2026-10-02",
		"product=global&start=2026-02-30&end=2026-03-01", "product=global&start=2026-10-02&end=2026-10-01",
		"product=global&start=2024-01-01&end=2026-01-01", "product=01&start=2026-10-01&end=2026-10-02"}
	for _, query := range queries {
		assertStatus(test, &fakeStore{}, "/api/v1/history?"+query, 400)
	}
}

func TestInvalidPagination(test *testing.T) {
	queries := []string{"limit=0", "limit=201", "limit=no", "after=-1", "after=", "after=9223372036854775808"}
	for _, query := range queries {
		assertStatus(test, &fakeStore{}, "/api/v1/products?"+query, 400)
	}
}

func assertStatus(test *testing.T, store Store, path string, expected int) {
	test.Helper()
	response := getResponse(store, path)
	if response.Code != expected {
		test.Fatalf("%s: %d, expected %d: %s", path, response.Code, expected, response.Body)
	}
}

func TestErrorsHideInternalDetails(test *testing.T) {
	path := "/api/v1/history?product=12&start=2026-10-01&end=2026-10-02"
	assertStatus(test, &fakeStore{failure: ErrNotFound}, path, 404)
	response := getResponse(&fakeStore{failure: errors.New("private database detail")}, path)
	if response.Code != 500 || strings.Contains(response.Body.String(), "private") {
		test.Fatal("internal details leaked")
	}
}
