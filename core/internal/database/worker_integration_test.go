package database

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"inflacao-real/core/internal/ingestion"
)

func workerPayload(t *testing.T, example string, number int, mutate func(map[string]any)) []byte {
	t.Helper()
	raw, err := os.ReadFile("../../../docs/contracts/ingestion/v1/examples/" + example + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	doc["submission_id"] = fmt.Sprintf("00000000-0000-4000-8000-%012d", number)
	if mutate != nil {
		mutate(doc)
	}
	raw, err = json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func firstObservation(doc map[string]any) map[string]any {
	return doc["observations"].([]any)[0].(map[string]any)
}

func enqueueWorkerPayload(t *testing.T, ctx context.Context, db *sql.DB, raw []byte) int64 {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	inserted, err := (ingestion.PostgresStore{DB: db}).Enqueue(ctx, ingestion.Submission{
		ScraperID: doc["scraper_id"].(string), SubmissionID: doc["submission_id"].(string),
		ShopID: doc["shop_id"].(string), SchemaVersion: doc["schema_version"].(string),
		CreatedAt: doc["created_at"].(string), Payload: raw,
	})
	if err != nil || !inserted {
		t.Fatalf("enqueue worker fixture: inserted=%t, error=%v", inserted, err)
	}
	var id int64
	if err := db.QueryRowContext(ctx, `SELECT id FROM ingestion_payloads WHERE scraper_id=$1 AND submission_id=$2`,
		doc["scraper_id"], doc["submission_id"]).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func registerWorkerShop(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `INSERT INTO shops (id,name,website_url)
		VALUES ('mercado-exemplo','Mercado Exemplo','https://mercado.example.com')`); err != nil {
		t.Fatal(err)
	}
}

func workerForTest(t *testing.T, db *sql.DB) *ingestion.Worker {
	t.Helper()
	w, err := ingestion.NewWorker(db, ingestion.DefaultWorkerConfig())
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func processWorkerPayload(t *testing.T, ctx context.Context, w *ingestion.Worker) {
	t.Helper()
	if found, err := w.ProcessNext(ctx); err != nil || !found {
		t.Fatalf("process next: found=%t, error=%v", found, err)
	}
}

func assertWorkerSQL(t *testing.T, ctx context.Context, db *sql.DB, query string, args ...any) {
	t.Helper()
	var ok bool
	if err := db.QueryRowContext(ctx, query, args...).Scan(&ok); err != nil || !ok {
		t.Fatalf("worker invariant failed: %s; result=%t, error=%v", query, ok, err)
	}
}

func TestWorkerPersistsOffersProvenanceAndObservationIdentity(t *testing.T) {
	ctx, db := testDatabase(t)
	registerWorkerShop(t, ctx, db)
	w := workerForTest(t, db)
	// Start at the HTTP boundary: acceptance precedes background normalization.
	raw := workerPayload(t, "submission", 1, nil)
	router, err := ingestion.NewRouter(ingestion.PostgresStore{DB: db}, ingestion.Auth{
		Token: "secret", ScraperID: "collector-sp-01", ShopIDs: []string{"mercado-exemplo"},
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingestion/submissions", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != http.StatusAccepted {
		t.Fatalf("HTTP ingestion returned %d: %s", response.Code, response.Body.String())
	}
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=0 FROM price_observations`)
	processWorkerPayload(t, ctx, w)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=1 AND bool_and(status='processed' AND attempts=1
		AND processed_at IS NOT NULL AND last_error IS NULL AND payload IS NOT NULL) FROM ingestion_payloads`)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=2 AND bool_and(
		observed_at='2026-10-27T12:00:00Z'::timestamptz
		AND package_quantity=5 AND package_unit='kg' AND package_count=1
		AND seller_id='retailer-direct' AND store_id='sp-centro' AND postal_code='01001000'
		AND city='São Paulo' AND state='SP' AND channel='delivery' AND availability='in_stock'
		AND source_url='https://mercado.example.com/api/products/arroz-5kg?store=sp-centro'
		AND collection_method='api' AND offer_count=2 AND price_currency='BRL' AND price_basis='package'
		AND coupon_required=false AND minimum_quantity=1) FROM price_observations`)
	assertWorkerSQL(t, ctx, db, `SELECT price_amount=24.90 AND reference_amount=27.90
		AND payment_method='any' AND membership_required=false AND membership_program IS NULL
		AND installments IS NULL AND condition_description='Oferta válida para a loja selecionada.'
		FROM price_observations WHERE offer_index=0`)
	assertWorkerSQL(t, ctx, db, `SELECT price_amount=22.90 AND reference_amount IS NULL
		AND payment_method='pix' AND membership_required=true AND membership_program='Clube Exemplo'
		FROM price_observations WHERE offer_index=1`)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=1 AND bool_and(run_id='a34040d4-ec04-48b6-b7d8-4bd42222f989'
		AND collector_name='mercado-exemplo' AND collector_version='0.1.0') FROM observation_identities`)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=1 AND bool_and(product_id IS NOT NULL) FROM shop_products`)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=1 FROM products`)
	assertWorkerSQL(t, ctx, db, `SELECT bool_and(product_id IS NOT NULL AND product_match_method='created_v1') FROM price_observations`)

	emptyID := enqueueWorkerPayload(t, ctx, db, workerPayload(t, "unavailable", 2, func(doc map[string]any) {
		firstObservation(doc)["product"].(map[string]any)["package"] = nil
	}))
	processWorkerPayload(t, ctx, w)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=1 AND bool_and(offer_count=0 AND offer_index=0
		AND price_amount IS NULL AND payment_method IS NULL AND coupon_required IS NULL
		AND package_quantity IS NULL AND package_unit IS NULL AND package_count IS NULL
		AND availability='out_of_stock' AND store_id IS NULL AND channel='unknown')
		FROM price_observations WHERE payload_id=$1`, emptyID)

	// Raw-payload cleanup must not remove cross-submission deduplication.
	if _, err := db.ExecContext(ctx, `UPDATE ingestion_payloads SET payload=NULL,payload_removed_at=now()`); err != nil {
		t.Fatal(err)
	}
	rebatchID := enqueueWorkerPayload(t, ctx, db, workerPayload(t, "submission", 3, func(doc map[string]any) {
		doc["run_id"] = "00000000-0000-4000-8000-000000000099"
		doc["collector"].(map[string]any)["version"] = "0.2.0"
	}))
	processWorkerPayload(t, ctx, w)
	assertWorkerSQL(t, ctx, db, `SELECT status='processed' FROM ingestion_payloads WHERE id=$1`, rebatchID)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=3 FROM price_observations`)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=2 FROM observation_identities`)
	// Independent scrapers retain independent measurements even with the same ID.
	enqueueWorkerPayload(t, ctx, db, workerPayload(t, "submission", 4, func(doc map[string]any) {
		doc["scraper_id"] = "collector-sp-02"
	}))
	processWorkerPayload(t, ctx, w)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=5 FROM price_observations`)

	// A delayed upload adds history without rolling current metadata backward.
	lateID := enqueueWorkerPayload(t, ctx, db, workerPayload(t, "submission", 5, func(doc map[string]any) {
		o := firstObservation(doc)
		o["observation_id"] = "00000000-0000-4000-8000-000000000005"
		o["observed_at"] = "2026-10-26T12:00:00Z"
		o["product"].(map[string]any)["name"] = "Older listing name"
	}))
	processWorkerPayload(t, ctx, w)
	assertWorkerSQL(t, ctx, db, `SELECT name='Arroz branco tipo 1 — pacote 5 kg'
		AND metadata_observed_at='2026-10-27T12:00:00Z'::timestamptz
		FROM shop_products WHERE source_product_id='sku-000123'`)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=2 AND bool_and(observed_at='2026-10-26T12:00:00Z'::timestamptz)
		FROM price_observations WHERE payload_id=$1`, lateID)
}

func TestWorkerRollsBackConflictsAndInvalidDomainData(t *testing.T) {
	ctx, db := testDatabase(t)
	registerWorkerShop(t, ctx, db)
	w := workerForTest(t, db)
	enqueueWorkerPayload(t, ctx, db, workerPayload(t, "submission", 1, nil))
	processWorkerPayload(t, ctx, w)
	for i, failure := range []string{"conflict", "GTIN", "reference"} {
		raw := workerPayload(t, "submission", i+2, func(doc map[string]any) {
			o := firstObservation(doc)
			// Insert a new observation before the bad one to verify batch rollback.
			clone, _ := json.Marshal(o)
			var good map[string]any
			if err := json.Unmarshal(clone, &good); err != nil {
				t.Fatal(err)
			}
			good["observation_id"] = "00000000-0000-4000-8000-000000000001"
			good["product"].(map[string]any)["source_product_id"] = "brand-new-listing"
			// Sort earlier than the failing observation in the worker's lock order.
			good["availability"] = "in_stock"
			o["availability"] = "unknown"
			if failure != "conflict" {
				o["observation_id"] = "00000000-0000-4000-8000-000000000002"
			}
			switch failure {
			case "GTIN":
				o["product"].(map[string]any)["gtin"] = "4006381333932"
			case "reference":
				o["offers"].([]any)[0].(map[string]any)["reference_amount"] = "24.89"
			}
			doc["observations"] = []any{good, o}
		})
		id := enqueueWorkerPayload(t, ctx, db, raw)
		processWorkerPayload(t, ctx, w)
		assertWorkerSQL(t, ctx, db, `SELECT status='failed' AND attempts=1 AND failed_at IS NOT NULL
			AND processed_at IS NULL AND last_error IS NOT NULL AND payload IS NOT NULL
			FROM ingestion_payloads WHERE id=$1`, id)
		assertWorkerSQL(t, ctx, db, `SELECT count(*)=2 FROM price_observations`)
		assertWorkerSQL(t, ctx, db, `SELECT count(*)=1 FROM observation_identities`)
		assertWorkerSQL(t, ctx, db, `SELECT count(*)=1 FROM shop_products`)
		assertWorkerSQL(t, ctx, db, `SELECT count(*)=1 FROM products`)
	}
	// A bad payload must not prevent the next independent submission succeeding.
	enqueueWorkerPayload(t, ctx, db, workerPayload(t, "unavailable", 9, nil))
	processWorkerPayload(t, ctx, w)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=3 FROM price_observations`)
}

func TestWorkerRetriesFailedCommitAndSupportsDeliberateReplay(t *testing.T) {
	ctx, db := testDatabase(t)
	registerWorkerShop(t, ctx, db)
	config := ingestion.DefaultWorkerConfig()
	config.MaxAttempts = 2
	w, err := ingestion.NewWorker(db, config)
	if err != nil {
		t.Fatal(err)
	}
	// Fail at transaction commit, after identity, listing and offer writes.
	if _, err := db.ExecContext(ctx, `CREATE FUNCTION fail_worker_commit() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'temporary storage failure'; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `CREATE CONSTRAINT TRIGGER fail_worker_commit
		AFTER INSERT ON price_observations DEFERRABLE INITIALLY DEFERRED
		FOR EACH ROW EXECUTE FUNCTION fail_worker_commit()`); err != nil {
		t.Fatal(err)
	}
	id := enqueueWorkerPayload(t, ctx, db, workerPayload(t, "submission", 1, nil))
	processWorkerPayload(t, ctx, w)
	assertWorkerSQL(t, ctx, db, `SELECT status='pending' AND attempts=1 AND next_attempt_at>now()
		AND last_error LIKE '%temporary storage failure%' FROM ingestion_payloads WHERE id=$1`, id)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=0 FROM price_observations`)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=0 FROM observation_identities`)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=0 FROM shop_products`)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=0 FROM products`)
	if found, err := w.ProcessNext(ctx); found || err != nil {
		t.Fatalf("retried before backoff elapsed: %t, %v", found, err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE ingestion_payloads SET next_attempt_at=now() WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	processWorkerPayload(t, ctx, w)
	assertWorkerSQL(t, ctx, db, `SELECT status='failed' AND attempts=2 AND failed_at IS NOT NULL
		AND payload IS NOT NULL FROM ingestion_payloads WHERE id=$1`, id)
	if _, err := db.ExecContext(ctx, `DROP TRIGGER fail_worker_commit ON price_observations`); err != nil {
		t.Fatal(err)
	}
	if found, err := w.ProcessNext(ctx); found || err != nil {
		t.Fatalf("terminal failure retried automatically: %t, %v", found, err)
	}
	// Operator replay preserves the immutable body and identity, resetting only queue state.
	if _, err := db.ExecContext(ctx, `UPDATE ingestion_payloads SET status='pending',failed_at=NULL,
		attempts=0,next_attempt_at=now(),last_error=NULL WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	processWorkerPayload(t, ctx, w)
	assertWorkerSQL(t, ctx, db, `SELECT status='processed' AND attempts=1 AND failed_at IS NULL
		AND last_error IS NULL FROM ingestion_payloads WHERE id=$1`, id)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=2 FROM price_observations`)
}

func TestWorkerSkipsLockedAndDeferredRowsAcrossConcurrentWorkers(t *testing.T) {
	ctx, db := testDatabase(t)
	registerWorkerShop(t, ctx, db)
	ids := make([]int64, 8)
	for i := range ids {
		ids[i] = enqueueWorkerPayload(t, ctx, db, workerPayload(t, "submission", i+1, func(doc map[string]any) {
			firstObservation(doc)["observation_id"] = fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1)
		}))
	}
	// Equal receipt times exercise the stable ID tie-breaker.
	if _, err := db.ExecContext(ctx, `UPDATE ingestion_payloads SET received_at='2026-10-27T12:00:00Z'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE ingestion_payloads SET next_attempt_at=now()+interval '1 hour' WHERE id=$1`, ids[1]); err != nil {
		t.Fatal(err)
	}
	lock, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback()
	if _, err := lock.ExecContext(ctx, `SELECT id FROM ingestion_payloads WHERE id=$1 FOR UPDATE`, ids[0]); err != nil {
		t.Fatal(err)
	}
	w := workerForTest(t, db)
	processWorkerPayload(t, ctx, w)
	assertWorkerSQL(t, ctx, db, `SELECT status='processed' FROM ingestion_payloads WHERE id=$1`, ids[2])
	var wg sync.WaitGroup
	errorsCh := make(chan error, 4)
	for range 4 {
		other := workerForTest(t, db)
		wg.Go(func() {
			for {
				found, err := other.ProcessNext(ctx)
				if err != nil {
					errorsCh <- err
					return
				}
				if !found {
					return
				}
			}
		})
	}
	wg.Wait()
	close(errorsCh)
	for err := range errorsCh {
		t.Fatal(err)
	}
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=6 AND bool_and(attempts=1) FROM ingestion_payloads WHERE status='processed'`)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=12 FROM price_observations`)
	// Releasing an interrupted transaction makes its pending payload available again.
	if err := lock.Rollback(); err != nil {
		t.Fatal(err)
	}
	processWorkerPayload(t, ctx, w)
	assertWorkerSQL(t, ctx, db, `SELECT status='processed' FROM ingestion_payloads WHERE id=$1`, ids[0])
	if _, err := db.ExecContext(ctx, `UPDATE ingestion_payloads SET next_attempt_at=now() WHERE id=$1`, ids[1]); err != nil {
		t.Fatal(err)
	}
	processWorkerPayload(t, ctx, w)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=8 AND bool_and(status='processed' AND attempts=1) FROM ingestion_payloads`)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=16 FROM price_observations`)

	// Competing rebatches of the same measurement must all succeed with no extra offers.
	for i := range 4 {
		enqueueWorkerPayload(t, ctx, db, workerPayload(t, "submission", i+20, func(doc map[string]any) {
			firstObservation(doc)["observation_id"] = "00000000-0000-4000-8000-000000000001"
		}))
	}
	results := make(chan error, 4)
	for range 4 {
		other := workerForTest(t, db)
		wg.Go(func() {
			found, err := other.ProcessNext(ctx)
			if err == nil && !found {
				err = errors.New("rebatch was not selected")
			}
			results <- err
		})
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=12 AND bool_and(status='processed') FROM ingestion_payloads`)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=16 FROM price_observations`)
}

func TestWorkerMissingShopRetriesAndRunStartsImmediately(t *testing.T) {
	ctx, db := testDatabase(t)
	id := enqueueWorkerPayload(t, ctx, db, workerPayload(t, "submission", 1, nil))
	config := ingestion.DefaultWorkerConfig()
	config.PollInterval = time.Hour
	w, err := ingestion.NewWorker(db, config)
	if err != nil {
		t.Fatal(err)
	}
	processWorkerPayload(t, ctx, w)
	assertWorkerSQL(t, ctx, db, `SELECT status='pending' AND attempts=1 AND next_attempt_at>now()
		AND last_error LIKE '%not registered%' FROM ingestion_payloads WHERE id=$1`, id)
	registerWorkerShop(t, ctx, db)
	if _, err := db.ExecContext(ctx, `UPDATE ingestion_payloads SET next_attempt_at=now() WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	workerCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); w.Run(workerCtx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("worker did not stop on cancellation")
		}
	})
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var processed bool
		if err := db.QueryRowContext(ctx, `SELECT status='processed' FROM ingestion_payloads WHERE id=$1`, id).Scan(&processed); err != nil {
			t.Fatal(err)
		}
		if processed {
			break
		}
		select {
		case <-deadline.C:
			t.Fatal("Run waited for its one-hour poll interval instead of starting immediately")
		case <-ticker.C:
		}
	}
	assertWorkerSQL(t, ctx, db, `SELECT attempts=2 AND last_error IS NULL FROM ingestion_payloads WHERE id=$1`, id)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=2 FROM price_observations`)
}

func TestWorkerCancellationRollsBackAndRecovers(t *testing.T) {
	ctx, db := testDatabase(t)
	registerWorkerShop(t, ctx, db)
	id := enqueueWorkerPayload(t, ctx, db, workerPayload(t, "submission", 1, nil))
	blocker, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback()
	if _, err := blocker.ExecContext(ctx, `SELECT pg_advisory_xact_lock(81234)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `CREATE FUNCTION block_worker() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN PERFORM pg_advisory_xact_lock(81234); RETURN NEW; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TRIGGER block_worker BEFORE INSERT ON price_observations
		FOR EACH ROW EXECUTE FUNCTION block_worker()`); err != nil {
		t.Fatal(err)
	}
	w := workerForTest(t, db)
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := w.ProcessNext(workerCtx); done <- err }()
	// Wait for the actual database lock, rather than guessing a sleep duration.
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting bool
		if err := db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM pg_stat_activity
			WHERE datname=current_database() AND wait_event='advisory')`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("worker returned before blocking: %v", err)
		case <-deadline.C:
			t.Fatal("worker did not reach blocking insert")
		case <-ticker.C:
		}
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled worker returned %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not stop after cancellation")
	}
	assertWorkerSQL(t, ctx, db, `SELECT status='pending' AND attempts=0 AND last_error IS NULL
		FROM ingestion_payloads WHERE id=$1`, id)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=0 FROM price_observations`)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=0 FROM observation_identities`)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=0 FROM shop_products`)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=0 FROM products`)
	if err := blocker.Rollback(); err != nil {
		t.Fatal(err)
	}
	processWorkerPayload(t, ctx, w)
	assertWorkerSQL(t, ctx, db, `SELECT status='processed' AND attempts=1 FROM ingestion_payloads WHERE id=$1`, id)
	assertWorkerSQL(t, ctx, db, `SELECT count(*)=2 FROM price_observations`)
}
