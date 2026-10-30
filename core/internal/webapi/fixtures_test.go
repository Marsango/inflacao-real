package webapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"inflacao-real/core/internal/database"
	"inflacao-real/core/internal/ingestion"
	"inflacao-real/core/internal/insights"
	"inflacao-real/core/internal/testdatabase"
)

type fixture struct {
	test   *testing.T
	ctx    context.Context
	pool   *sql.DB
	router *gin.Engine
	worker *ingestion.Worker
}

func timeForTest() time.Time { return time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC) }

func newFixture(test *testing.T) fixture {
	test.Helper()
	ctx, pool := testdatabase.New(test, database.Open, database.Migrate)
	worker, err := ingestion.NewWorker(pool, ingestion.DefaultWorkerConfig())
	requireSuccess(test, err)
	router, err := ingestion.NewRouter(ingestion.PostgresStore{DB: pool}, ingestion.Auth{Token: "test", ScraperID: "collector-sp-01", ShopIDs: []string{"mercado-exemplo"}})
	requireSuccess(test, err)
	RegisterRoutes(router, PostgresStore{DB: pool})
	result := fixture{test, ctx, pool, router, worker}
	result.execute(`INSERT INTO shops VALUES ('mercado-exemplo','Mercado Exemplo','https://mercado.example.com')`)
	return result
}

func requireSuccess(test *testing.T, err error) {
	test.Helper()
	if err != nil {
		test.Fatal(err)
	}
}

func (fixture fixture) execute(query string, args ...any) {
	fixture.test.Helper()
	_, err := fixture.pool.ExecContext(fixture.ctx, query, args...)
	requireSuccess(fixture.test, err)
}

func (fixture fixture) submit(number int, mutate func(map[string]any)) {
	fixture.test.Helper()
	raw, err := os.ReadFile("../../../docs/contracts/ingestion/v1/examples/submission.json")
	requireSuccess(fixture.test, err)
	var document map[string]any
	requireSuccess(fixture.test, json.Unmarshal(raw, &document))
	document["submission_id"] = fmt.Sprintf("00000000-0000-4000-8000-%012d", number)
	observation := document["observations"].([]any)[0].(map[string]any)
	observation["observation_id"] = document["submission_id"]
	if mutate != nil {
		mutate(observation)
	}
	raw, err = json.Marshal(document)
	requireSuccess(fixture.test, err)
	fixture.ingest(raw)
}

func (fixture fixture) ingest(raw []byte) {
	fixture.test.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/ingestion/submissions", bytes.NewReader(raw))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer test")
	response := httptest.NewRecorder()
	fixture.router.ServeHTTP(response, request)
	if response.Code != 202 {
		fixture.test.Fatalf("ingest: %d %s", response.Code, response.Body)
	}
	found, err := fixture.worker.ProcessNext(fixture.ctx)
	requireSuccess(fixture.test, err)
	if !found {
		fixture.test.Fatal("ingestion was not processed")
	}
}

func (fixture fixture) get(path string, target any) {
	fixture.test.Helper()
	response := httptest.NewRecorder()
	fixture.router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
	if response.Code != 200 {
		fixture.test.Fatalf("GET %s: %d %s", path, response.Code, response.Body)
	}
	requireSuccess(fixture.test, json.Unmarshal(response.Body.Bytes(), target))
}

func (fixture fixture) history(product, start, end string) insights.History {
	fixture.test.Helper()
	var history insights.History
	fixture.get(fmt.Sprintf("/api/v1/history?product=%s&start=%s&end=%s", product, start, end), &history)
	return history
}

func setPrice(observation map[string]any, observedAt, price string) {
	observation["observed_at"] = observedAt
	offer := observation["offers"].([]any)[0].(map[string]any)
	offer["amount"], offer["reference_amount"] = price, nil
}

func assertPrice(test *testing.T, value *int64, expected int64) {
	test.Helper()
	if value == nil || *value != expected {
		test.Fatalf("expected %d, got %v", expected, value)
	}
}

func setUnitPrice(observation map[string]any, basis string) {
	observation["product"].(map[string]any)["package"] = nil
	observation["offers"].([]any)[0].(map[string]any)["basis"] = basis
	observation["offers"] = observation["offers"].([]any)[:1]
}
