package historycache

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

	"github.com/gin-gonic/gin"
	"inflacao-real/core/internal/database"
	"inflacao-real/core/internal/equivalence"
	"inflacao-real/core/internal/ingestion"
	"inflacao-real/core/internal/insights"
	"inflacao-real/core/internal/testdatabase"
	"inflacao-real/core/internal/webapi"
)

type databaseFixture struct {
	test    *testing.T
	context context.Context
	pool    *sql.DB
	router  *gin.Engine
	worker  *ingestion.Worker
	cache   *Store
}

func newDatabaseFixture(test *testing.T) databaseFixture {
	test.Helper()
	requestContext, pool := testdatabase.New(test, database.Open, database.Migrate)
	worker, err := ingestion.NewWorker(pool, ingestion.DefaultWorkerConfig())
	requireSuccess(test, err)
	router, err := ingestion.NewRouter(ingestion.PostgresStore{DB: pool}, ingestion.Auth{
		Token: "test", ScraperID: "collector-sp-01", ShopIDs: []string{"mercado-exemplo"}})
	requireSuccess(test, err)
	cache := newTestStore(test, webapi.PostgresStore{DB: pool}, DefaultConfig())
	webapi.RegisterRoutes(router, cache)
	_, err = pool.ExecContext(requestContext, `INSERT INTO shops VALUES ('mercado-exemplo','Mercado Exemplo','https://mercado.example.com')`)
	requireSuccess(test, err)
	return databaseFixture{test, requestContext, pool, router, worker, cache}
}

func (fixture databaseFixture) queue(number int, observedAt, amount, sourceID, gtin string) {
	fixture.test.Helper()
	raw, err := os.ReadFile("../../../docs/contracts/ingestion/v1/examples/submission.json")
	requireSuccess(fixture.test, err)
	var document map[string]any
	requireSuccess(fixture.test, json.Unmarshal(raw, &document))
	identity := fmt.Sprintf("00000000-0000-4000-8000-%012d", number)
	document["submission_id"] = identity
	observation := document["observations"].([]any)[0].(map[string]any)
	observation["observation_id"], observation["observed_at"] = identity, observedAt
	product := observation["product"].(map[string]any)
	product["source_product_id"], product["gtin"] = sourceID, gtin
	offer := observation["offers"].([]any)[0].(map[string]any)
	offer["amount"], offer["reference_amount"] = amount, nil
	raw, err = json.Marshal(document)
	requireSuccess(fixture.test, err)
	fixture.enqueue(raw)
}

func (fixture databaseFixture) enqueue(raw []byte) {
	fixture.test.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/ingestion/submissions", bytes.NewReader(raw))
	request.Header.Set("Authorization", "Bearer test")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	fixture.router.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		fixture.test.Fatalf("ingestion rejected: %d %s", response.Code, response.Body)
	}
}

func (fixture databaseFixture) process() {
	fixture.test.Helper()
	found, err := fixture.worker.ProcessNext(fixture.context)
	requireSuccess(fixture.test, err)
	if !found {
		fixture.test.Fatal("no queued submission found")
	}
}

func (fixture databaseFixture) history(product string) insights.History {
	fixture.test.Helper()
	path := "/api/v1/history?product=" + product + "&start=2026-10-27&end=2026-10-28"
	response := httptest.NewRecorder()
	fixture.router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		fixture.test.Fatalf("unexpected history response: %d %s", response.Code, response.Body)
	}
	var history insights.History
	requireSuccess(fixture.test, json.Unmarshal(response.Body.Bytes(), &history))
	return history
}

func TestCommittedIngestionAndEquivalenceInvalidateHistories(test *testing.T) {
	fixture := newDatabaseFixture(test)
	verifyQueuedAndProcessedHistory(fixture)
	fixture.queue(2, "2026-10-28T12:00:00Z", "29.90", "first", "96385074")
	assertLatestPrice(test, fixture.history("1"), 2490)
	fixture.process()
	assertLatestPrice(test, fixture.history("1"), 2990)
	fixture.queue(3, "2026-10-27T12:00:00Z", "19.90", "second", "4006381333931")
	fixture.process()
	verifyEquivalenceInvalidation(fixture)
}

func verifyQueuedAndProcessedHistory(fixture databaseFixture) {
	fixture.test.Helper()
	assertProductCount(fixture.test, fixture.history("global"), 0)
	fixture.queue(1, "2026-10-27T12:00:00Z", "24.90", "first", "96385074")
	assertProductCount(fixture.test, fixture.history("global"), 0)
	if statistics := fixture.cache.Stats(); statistics.Calculations != 1 {
		fixture.test.Fatalf("enqueue invalidated an unchanged history: %+v", statistics)
	}
	fixture.process()
	assertProductCount(fixture.test, fixture.history("global"), 1)
	assertLatestPrice(fixture.test, fixture.history("1"), 2490)
}

func verifyEquivalenceInvalidation(fixture databaseFixture) {
	fixture.test.Helper()
	assertProductCount(fixture.test, fixture.history("global"), 2)
	assertLatestPrice(fixture.test, fixture.history("2"), 1990)
	revision, err := webapi.PostgresStore{DB: fixture.pool}.Revision(fixture.context)
	requireSuccess(fixture.test, err)
	decision := equivalence.Decision{Revision: revision, Action: "approve", First: 1, Second: 2,
		Reason: "Reviewed source identity", SourceVerified: true}
	requireSuccess(fixture.test, equivalence.Apply(fixture.context, fixture.pool, "owner", decision))
	assertProductCount(fixture.test, fixture.history("global"), 1)
	assertLatestPrice(fixture.test, fixture.history("2"), 2990)
	verifyDissolutionInvalidation(fixture)
}

func verifyDissolutionInvalidation(fixture databaseFixture) {
	fixture.test.Helper()
	revision, err := webapi.PostgresStore{DB: fixture.pool}.Revision(fixture.context)
	requireSuccess(fixture.test, err)
	decision := equivalence.Decision{Revision: revision, Action: "dissolve", First: 1, Reason: "Correct source identity"}
	requireSuccess(fixture.test, equivalence.Apply(fixture.context, fixture.pool, "owner", decision))
	assertProductCount(fixture.test, fixture.history("global"), 2)
	assertLatestPrice(fixture.test, fixture.history("2"), 1990)
}

func TestUncommittedAndRolledBackChangesKeepCachedHistory(test *testing.T) {
	fixture := newDatabaseFixture(test)
	fixture.queue(1, "2026-10-27T12:00:00Z", "24.90", "first", "96385074")
	fixture.process()
	assertLatestPrice(test, fixture.history("1"), 2490)
	transaction, err := fixture.pool.BeginTx(fixture.context, nil)
	requireSuccess(test, err)
	test.Cleanup(func() { transaction.Rollback() })
	_, err = transaction.ExecContext(fixture.context, `UPDATE price_observations SET price_amount=99; UPDATE catalog_decisions SET revision=revision+1`)
	requireSuccess(test, err)
	assertLatestPrice(test, fixture.history("1"), 2490)
	requireSuccess(test, transaction.Rollback())
	assertLatestPrice(test, fixture.history("1"), 2490)
	if statistics := fixture.cache.Stats(); statistics.Calculations != 1 {
		test.Fatalf("uncommitted or rolled back changes invalidated cache: %+v", statistics)
	}
}

func assertLatestPrice(test *testing.T, history insights.History, expected int64) {
	test.Helper()
	if history.LatestPriceMinor == nil || *history.LatestPriceMinor != expected {
		test.Fatalf("expected price %d, got %+v", expected, history)
	}
}

func assertProductCount(test *testing.T, history insights.History, expected int) {
	test.Helper()
	if history.Counts.Products != expected {
		test.Fatalf("expected %d products, got %+v", expected, history.Counts)
	}
}
