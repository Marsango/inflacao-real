package database

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"inflacao-real/core/internal/ingestion"
)

func TestIngestionHTTPPersistence(t *testing.T) {
	ctx, db := testDatabase(t)
	body, err := os.ReadFile("../../../docs/contracts/ingestion/v1/examples/submission.json")
	if err != nil {
		t.Fatal(err)
	}
	router, err := ingestion.NewRouter(ingestion.PostgresStore{DB: db}, ingestion.Auth{Token: "secret", ScraperID: "collector-sp-01", ShopIDs: []string{"mercado-exemplo"}})
	if err != nil {
		t.Fatal(err)
	}
	submit := func(body []byte) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/ingestion/submissions", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer secret")
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}
	// Competing requests must produce exactly one committed queue entry.
	const parallel = 12
	responses := make(chan *httptest.ResponseRecorder, parallel)
	var wg sync.WaitGroup
	for range parallel {
		wg.Go(func() { responses <- submit(body) })
	}
	wg.Wait()
	close(responses)
	accepted := 0
	for response := range responses {
		if response.Code == http.StatusAccepted {
			accepted++
		} else if response.Code != http.StatusOK {
			t.Fatalf("unexpected status %d: %s", response.Code, response.Body.String())
		}
		if response.Body.Len() != 0 {
			t.Fatal("successful response must be empty")
		}
	}
	if accepted != 1 {
		t.Fatalf("accepted %d copies; want 1", accepted)
	}
	var valid bool
	if err := db.QueryRowContext(ctx, `SELECT count(*) = 1 AND bool_and(
		status = 'pending' AND attempts = 0 AND processed_at IS NULL
		AND failed_at IS NULL AND last_error IS NULL AND payload_removed_at IS NULL
		AND received_at IS NOT NULL AND next_attempt_at = received_at
		AND scraper_id = 'collector-sp-01' AND shop_id = 'mercado-exemplo'
		AND schema_version = '1.0' AND created_at = ($1::jsonb->>'created_at')::timestamptz
		AND submission_id = ($1::jsonb->>'submission_id')::uuid AND payload = $1::jsonb)
		FROM ingestion_payloads`, string(body)).Scan(&valid); err != nil || !valid {
		t.Fatalf("incorrect queued payload/defaults: %t, %v", valid, err)
	}
	// Identity-only retries cannot replace content or reset retry/terminal state.
	changed := bytes.Replace(body, []byte(`"24.90"`), []byte(`"25.90"`), 1)
	for _, state := range []string{"pending", "failed", "processed"} {
		if _, err := db.ExecContext(ctx, `UPDATE ingestion_payloads SET status=$1::text, attempts=3,
			next_attempt_at=now()+interval '1 hour', last_error='original diagnostics',
			processed_at=CASE WHEN $1='processed' THEN now() END,
			failed_at=CASE WHEN $1='failed' THEN now() END`, state); err != nil {
			t.Fatal(err)
		}
		response := submit(changed)
		if response.Code != http.StatusOK {
			t.Fatalf("retry in %s returned %d", state, response.Code)
		}
		if err := db.QueryRowContext(ctx, `SELECT status=$1 AND attempts=3
			AND last_error='original diagnostics' AND next_attempt_at>now()
			AND payload=$2::jsonb FROM ingestion_payloads`, state, string(body)).Scan(&valid); err != nil || !valid {
			t.Fatalf("retry modified %s record: %t, %v", state, valid, err)
		}
	}
	if _, err := db.ExecContext(ctx, `UPDATE ingestion_payloads SET payload=NULL,payload_removed_at=now()`); err != nil {
		t.Fatal(err)
	}
	if response := submit(body); response.Code != http.StatusOK {
		t.Fatalf("retry after cleanup returned %d", response.Code)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*)=1 AND bool_and(payload IS NULL AND status='processed') FROM ingestion_payloads`).Scan(&valid); err != nil || !valid {
		t.Fatalf("cleanup identity lost: %t, %v", valid, err)
	}
}

func TestIngestionDoesNotAcknowledgeFailedCommit(t *testing.T) {
	ctx, db := testDatabase(t)
	// A deferred trigger fails at commit, after the INSERT itself succeeded.
	if _, err := db.ExecContext(ctx, `CREATE FUNCTION reject_ingestion_commit() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'private commit failure'; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `CREATE CONSTRAINT TRIGGER reject_commit AFTER INSERT ON ingestion_payloads
		DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_ingestion_commit()`); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile("../../../docs/contracts/ingestion/v1/examples/submission.json")
	if err != nil {
		t.Fatal(err)
	}
	router, err := ingestion.NewRouter(ingestion.PostgresStore{DB: db}, ingestion.Auth{Token: "secret", ScraperID: "collector-sp-01", ShopIDs: []string{"mercado-exemplo"}})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingestion/submissions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != http.StatusInternalServerError || response.Body.String() != `{"error":"ingestion_unavailable"}` {
		t.Fatalf("failed commit acknowledged or exposed: %d %s", response.Code, response.Body.String())
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM ingestion_payloads`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed commit left rows: %d, %v", count, err)
	}
}
