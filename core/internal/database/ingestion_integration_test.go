package database

import (
	"bytes"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"inflacao-real/core/internal/ingestion"
)

const queuedDefaultsSQL = `SELECT count(*) = 1 AND bool_and(
		status = 'pending' AND attempts = 0 AND processed_at IS NULL
		AND failed_at IS NULL AND last_error IS NULL AND payload_removed_at IS NULL
		AND received_at IS NOT NULL AND next_attempt_at = received_at
		AND scraper_id = 'collector-sp-01' AND shop_id = 'mercado-exemplo'
		AND schema_version = '1.0' AND created_at = ($1::jsonb->>'created_at')::timestamptz
		AND submission_id = ($1::jsonb->>'submission_id')::uuid AND payload = $1::jsonb)
		FROM ingestion_payloads`

const retryStateSQL = `UPDATE ingestion_payloads SET status=$1::text, attempts=3,
			next_attempt_at=now()+interval '1 hour', last_error='original diagnostics',
			processed_at=CASE WHEN $1='processed' THEN now() END,
			failed_at=CASE WHEN $1='failed' THEN now() END`

const unchangedRetrySQL = `SELECT status=$1 AND attempts=3
			AND last_error='original diagnostics' AND next_attempt_at>now()
			AND payload=$2::jsonb FROM ingestion_payloads`

func readExample(test *testing.T, name string) []byte {
	test.Helper()
	raw, err := os.ReadFile("../../../docs/contracts/ingestion/v1/examples/" + name + ".json")
	requireNoError(test, err)
	return raw
}

func ingestionRouter(test *testing.T, pool *sql.DB) http.Handler {
	test.Helper()
	auth := ingestion.Auth{Token: "secret", ScraperID: "collector-sp-01", ShopIDs: []string{"mercado-exemplo"}}
	router, err := ingestion.NewRouter(ingestion.PostgresStore{DB: pool}, auth)
	requireNoError(test, err)
	return router
}

func submitRequest(handler http.Handler, body []byte) *httptest.ResponseRecorder {
	incoming := httptest.NewRequest(http.MethodPost, "/api/v1/ingestion/submissions", bytes.NewReader(body))
	incoming.Header.Set("Authorization", "Bearer secret")
	incoming.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, incoming)
	return response
}

func requireHTTPStatus(test *testing.T, response *httptest.ResponseRecorder, expected int) {
	test.Helper()
	if response.Code != expected {
		test.Fatalf("HTTP ingestion returned %d: %s", response.Code, response.Body.String())
	}
}

type httpFixture struct {
	workerFixture
	handler http.Handler
	body    []byte
}

func newHTTPFixture(test *testing.T) httpFixture {
	fixture := newWorkerFixture(test)
	return httpFixture{fixture, ingestionRouter(test, fixture.database), readExample(test, "submission")}
}

func TestIngestionHTTPPersistence(test *testing.T) {
	fixture := newHTTPFixture(test)
	fixture.assertConcurrentAcceptance()
	fixture.assert(queuedDefaultsSQL, string(fixture.body))
	for _, state := range []string{"pending", "failed", "processed"} {
		fixture.assertImmutableRetry(state)
	}
	fixture.assertRetryAfterCleanup()
}

func concurrentSubmissions(handler http.Handler, body []byte, count int) <-chan *httptest.ResponseRecorder {
	responses := make(chan *httptest.ResponseRecorder, count)
	var workers sync.WaitGroup
	for range count {
		workers.Go(func() { responses <- submitRequest(handler, body) })
	}
	workers.Wait()
	close(responses)
	return responses
}

func (fixture httpFixture) assertConcurrentAcceptance() {
	accepted := 0
	for response := range concurrentSubmissions(fixture.handler, fixture.body, 12) {
		accepted += acceptedCopy(fixture.test, response)
	}
	if accepted != 1 {
		fixture.test.Fatalf("accepted %d copies; want 1", accepted)
	}
}

func acceptedCopy(test *testing.T, response *httptest.ResponseRecorder) int {
	test.Helper()
	if response.Body.Len() != 0 {
		test.Fatal("successful response must be empty")
	}
	if response.Code == http.StatusAccepted {
		return 1
	}
	requireHTTPStatus(test, response, http.StatusOK)
	return 0
}

func (fixture httpFixture) assertImmutableRetry(state string) {
	fixture.execute(retryStateSQL, state)
	changed := bytes.Replace(fixture.body, []byte(`"24.90"`), []byte(`"25.90"`), 1)
	response := submitRequest(fixture.handler, changed)
	requireHTTPStatus(fixture.test, response, http.StatusOK)
	fixture.assert(unchangedRetrySQL, state, string(fixture.body))
}

func (fixture httpFixture) assertRetryAfterCleanup() {
	fixture.execute(`UPDATE ingestion_payloads SET payload=NULL,payload_removed_at=now()`)
	response := submitRequest(fixture.handler, fixture.body)
	requireHTTPStatus(fixture.test, response, http.StatusOK)
	fixture.assert(`SELECT count(*)=1 AND bool_and(payload IS NULL AND status='processed') FROM ingestion_payloads`)
}

func TestIngestionDoesNotAcknowledgeFailedCommit(test *testing.T) {
	fixture := newHTTPFixture(test)
	fixture.execute(`CREATE FUNCTION reject_ingestion_commit() RETURNS trigger LANGUAGE plpgsql AS $$
        BEGIN RAISE EXCEPTION 'private commit failure'; END $$`)
	fixture.execute(`CREATE CONSTRAINT TRIGGER reject_commit AFTER INSERT ON ingestion_payloads
        DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_ingestion_commit()`)
	response := submitRequest(fixture.handler, fixture.body)
	requireHTTPStatus(test, response, http.StatusInternalServerError)
	if response.Body.String() != `{"error":"ingestion_unavailable"}` {
		test.Fatal("failed commit exposed")
	}
	fixture.assert(`SELECT count(*)=0 FROM ingestion_payloads`)
}

func TestKilogramHTTPObservationsReachHistoryWithoutDuplicateRetries(test *testing.T) {
	fixture := newHTTPFixture(test)
	fixture.registerShop()
	fixture.body = readExample(test, "per-kilogram")
	requireHTTPStatus(test, submitRequest(fixture.handler, fixture.body), http.StatusAccepted)
	fixture.assert(`SELECT count(*)=0 FROM price_observations`)
	fixture.process()
	fixture.assert(processedQueueSQL)
	requireHTTPStatus(test, submitRequest(fixture.handler, fixture.body), http.StatusOK)
	fixture.assertKilogramHistory()
	fixture.submitNextKilogramObservation()
	fixture.assert(`SELECT count(*)=2 AND count(DISTINCT observation_id)=2
        AND count(DISTINCT product_id)=1 AND count(DISTINCT shop_product_id)=1 FROM price_observations`)
	fixture.assert(`SELECT count(*)=1 AND bool_and(gtin IS NULL) FROM products`)
}

func (fixture httpFixture) assertKilogramHistory() {
	fixture.assert(`SELECT count(*)=1 AND bool_and(price_amount=6.99 AND price_basis='kg'
        AND reference_amount=7.99 AND package_quantity IS NULL AND package_unit IS NULL
        AND package_count IS NULL AND minimum_quantity=0.5 AND payment_method='any'
        AND membership_required=false AND coupon_required=false AND installments IS NULL
        AND seller_id='retailer-direct' AND store_id='sp-centro' AND postal_code='01001000'
        AND city='São Paulo' AND state='SP' AND channel='delivery' AND availability='in_stock'
        AND source_url='https://mercado.example.com/api/products/tomate?store=sp-centro'
        AND collection_method='api'
        AND observed_at='2026-10-27T12:00:00Z'::timestamptz AND offer_count=1
        AND condition_description LIKE '%peso final%') FROM price_observations`)
	fixture.assert(`SELECT count(*)=1 AND bool_and(product_url='https://mercado.example.com/products/tomate')
        FROM shop_products WHERE source_product_id='tomate-solto'`)
}

func (fixture httpFixture) submitNextKilogramObservation() {
	raw := workerPayload(fixture.test, "per-kilogram", 2, func(document map[string]any) {
		observationNumber(2)(document)
		firstObservation(document)["observed_at"] = "2026-10-28T12:00:00Z"
	})
	requireHTTPStatus(fixture.test, submitRequest(fixture.handler, raw), http.StatusAccepted)
	fixture.process()
	requireHTTPStatus(fixture.test, submitRequest(fixture.handler, raw), http.StatusOK)
	fixture.assert(`SELECT count(*)=2 AND bool_and(status='processed') FROM ingestion_payloads`)
}
