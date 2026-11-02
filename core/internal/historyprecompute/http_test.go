package historyprecompute

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"inflacao-real/core/internal/ingestion"
)

func TestRefreshAuthenticationAndValidation(test *testing.T) {
	fixture := newFixture(test)
	auth := ingestion.Auth{Token: "test", ScraperID: "collector", ShopIDs: []string{"shop"}}
	router, err := ingestion.NewRouter(ingestion.PostgresStore{DB: fixture.database}, auth)
	requireSuccess(test, err)
	RegisterRoutes(router, fixture.store, auth.Authenticate, auth.ScraperID, auth.ShopIDs)
	checkRefresh(test, router, `{}`, "test", 400)
	checkRefresh(test, router, `{"scraper_id":"collector","shop_id":"shop"}`, "", 401)
	checkRefresh(test, router, `{"scraper_id":"collector","shop_id":"other"}`, "test", 403)
	checkRefresh(test, router, `{"scraper_id":"other","shop_id":"shop"}`, "test", 403)
	checkRefresh(test, router, `{"scraper_id":"collector","shop_id":"shop","extra":1}`, "test", 400)
	checkRefresh(test, router, `{"scraper_id":"collector","shop_id":"shop"} {}`, "test", 400)
	checkRefresh(test, router, `{"scraper_id":"collector","shop_id":"shop"}`, "test", 202)
	var generation int
	requireSuccess(test, fixture.database.QueryRowContext(fixture.requestContext, `SELECT generation FROM history_refresh_requests`).Scan(&generation))
	if generation != 2 {
		test.Fatal("accepted refresh was not durable")
	}
}

func checkRefresh(test *testing.T, router http.Handler, body, token string, status int) {
	test.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/ingestion/history-refresh", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != status {
		test.Fatalf("refresh status %d, expected %d: %s", response.Code, status, response.Body)
	}
}
