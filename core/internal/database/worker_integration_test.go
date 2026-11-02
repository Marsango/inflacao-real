package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"inflacao-real/core/internal/dailysummary"
	"inflacao-real/core/internal/ingestion"
	"inflacao-real/core/internal/webapi"
)

const processedQueueSQL = `SELECT count(*)=1 AND bool_and(status='processed' AND attempts=1
		AND processed_at IS NOT NULL AND last_error IS NULL AND payload IS NOT NULL) FROM ingestion_payloads`

const observationProvenanceSQL = `SELECT count(*)=2 AND bool_and(
		observed_at='2026-10-27T12:00:00Z'::timestamptz
		AND package_quantity=5 AND package_unit='kg' AND package_count=1
		AND seller_id='retailer-direct' AND store_id='sp-centro' AND postal_code='01001000'
		AND city='São Paulo' AND state='SP' AND channel='delivery' AND availability='in_stock'
		AND source_url='https://mercado.example.com/api/products/arroz-5kg?store=sp-centro'
		AND collection_method='api' AND offer_count=2 AND price_currency='BRL' AND price_basis='package'
		AND coupon_required=false AND minimum_quantity=1) FROM price_observations`

const regularPriceSQL = `SELECT price_amount=24.90 AND reference_amount=27.90
		AND payment_method='any' AND membership_required=false AND membership_program IS NULL
		AND installments IS NULL AND condition_description='Oferta válida para a loja selecionada.'
		FROM price_observations WHERE offer_index=0`

const memberPriceSQL = `SELECT price_amount=22.90 AND reference_amount IS NULL
		AND payment_method='pix' AND membership_required=true AND membership_program='Clube Exemplo'
		FROM price_observations WHERE offer_index=1`

const collectorIdentitySQL = `SELECT count(*)=1 AND bool_and(run_id='a34040d4-ec04-48b6-b7d8-4bd42222f989'
		AND collector_name='mercado-exemplo' AND collector_version='0.1.0') FROM observation_identities`

const unavailableObservationSQL = `SELECT count(*)=1 AND bool_and(offer_count=0 AND offer_index=0
		AND price_amount IS NULL AND payment_method IS NULL AND coupon_required IS NULL
		AND package_quantity IS NULL AND package_unit IS NULL AND package_count IS NULL
		AND availability='out_of_stock' AND store_id IS NULL AND channel='unknown')
		FROM price_observations WHERE payload_id=$1`

const permanentFailureSQL = `SELECT status='failed' AND attempts=1 AND failed_at IS NOT NULL
			AND processed_at IS NULL AND last_error IS NOT NULL AND payload IS NOT NULL
			FROM ingestion_payloads WHERE id=$1`

func workerPayload(test *testing.T, example string, number int, mutate func(map[string]any)) []byte {
	test.Helper()
	raw := readExample(test, example)
	var document map[string]any
	requireNoError(test, json.Unmarshal(raw, &document))
	document["submission_id"] = fmt.Sprintf("00000000-0000-4000-8000-%012d", number)
	if mutate != nil {
		mutate(document)
	}
	result, err := json.Marshal(document)
	requireNoError(test, err)
	return result
}

func firstObservation(document map[string]any) map[string]any {
	return document["observations"].([]any)[0].(map[string]any)
}

func enqueueWorkerPayload(test *testing.T, requestContext context.Context, pool *sql.DB, raw []byte) int64 {
	test.Helper()
	var document map[string]any
	requireNoError(test, json.Unmarshal(raw, &document))
	store := ingestion.PostgresStore{DB: pool}
	inserted, err := store.Enqueue(requestContext, submissionMetadata(document, raw))
	if err != nil || !inserted {
		test.Fatalf("enqueue worker fixture: inserted=%t, error=%v", inserted, err)
	}
	var identity int64
	row := pool.QueryRowContext(requestContext, `SELECT id FROM ingestion_payloads WHERE scraper_id=$1 AND submission_id=$2`, document["scraper_id"], document["submission_id"])
	requireNoError(test, row.Scan(&identity))
	return identity
}

func submissionMetadata(document map[string]any, raw []byte) ingestion.Submission {
	return ingestion.Submission{ScraperID: document["scraper_id"].(string), SubmissionID: document["submission_id"].(string),
		ShopID: document["shop_id"].(string), SchemaVersion: document["schema_version"].(string),
		CreatedAt: document["created_at"].(string), Payload: raw}
}

func registerWorkerShop(test *testing.T, requestContext context.Context, pool *sql.DB) {
	test.Helper()
	if _, err := pool.ExecContext(requestContext, `INSERT INTO shops (id,name,website_url)
		VALUES ('mercado-exemplo','Mercado Exemplo','https://mercado.example.com')`); err != nil {
		test.Fatal(err)
	}
}

func workerForTest(test *testing.T, pool *sql.DB) *ingestion.Worker {
	test.Helper()
	worker, err := ingestion.NewWorker(pool, ingestion.DefaultWorkerConfig())
	if err != nil {
		test.Fatal(err)
	}
	return worker
}

func processWorkerPayload(test *testing.T, requestContext context.Context, worker *ingestion.Worker) {
	test.Helper()
	if found, err := worker.ProcessNext(requestContext); err != nil || !found {
		test.Fatalf("process next: found=%t, error=%v", found, err)
	}
}

func assertWorkerSQL(test *testing.T, requestContext context.Context, pool *sql.DB, query string, arguments ...any) {
	test.Helper()
	var ok bool
	if err := pool.QueryRowContext(requestContext, query, arguments...).Scan(&ok); err != nil || !ok {
		test.Fatalf("worker invariant failed: %s; result=%t, error=%v", query, ok, err)
	}
}

func TestWorkerPersistsOffersProvenanceAndObservationIdentity(test *testing.T) {
	fixture := newWorkerFixture(test)
	fixture.registerShop()
	fixture.submitInitialObservation()
	fixture.assertRegularMeasurement()
	fixture.checkUnavailableMeasurement()
	fixture.checkRebatchAfterCleanup()
	fixture.checkIndependentScraper()
	fixture.checkLateUpload()
}

func (fixture workerFixture) submitInitialObservation() {
	raw := workerPayload(fixture.test, "submission", 1, nil)
	response := submitRequest(ingestionRouter(fixture.test, fixture.database), raw)
	requireHTTPStatus(fixture.test, response, http.StatusAccepted)
	fixture.assert(`SELECT count(*)=0 FROM price_observations`)
	fixture.process()
}

func (fixture workerFixture) assertRegularMeasurement() {
	fixture.assert(processedQueueSQL)
	fixture.assert(observationProvenanceSQL)
	fixture.assert(regularPriceSQL)
	fixture.assert(memberPriceSQL)
	fixture.assert(collectorIdentitySQL)
	fixture.assert(`SELECT count(*)=1 AND bool_and(product_id IS NOT NULL) FROM shop_products`)
	fixture.assert(`SELECT count(*)=1 FROM products`)
	fixture.assert(`SELECT bool_and(product_id IS NOT NULL AND product_match_method='created_v1') FROM price_observations`)
}

func (fixture workerFixture) checkUnavailableMeasurement() {
	identity := fixture.enqueue("unavailable", 2, func(document map[string]any) { firstObservation(document)["product"].(map[string]any)["package"] = nil })
	fixture.process()
	fixture.assert(unavailableObservationSQL, identity)
}

func (fixture workerFixture) checkRebatchAfterCleanup() {
	fixture.execute(`UPDATE ingestion_payloads SET payload=NULL,payload_removed_at=now()`)
	identity := fixture.enqueue("submission", 3, func(document map[string]any) {
		document["run_id"] = "00000000-0000-4000-8000-000000000099"
		document["collector"].(map[string]any)["version"] = "0.2.0"
	})
	fixture.process()
	fixture.assert(`SELECT status='processed' FROM ingestion_payloads WHERE id=$1`, identity)
	fixture.assert(`SELECT count(*)=3 FROM price_observations`)
	fixture.assert(`SELECT count(*)=2 FROM observation_identities`)
}

func (fixture workerFixture) checkIndependentScraper() {
	fixture.enqueue("submission", 4, func(document map[string]any) { document["scraper_id"] = "collector-sp-02" })
	fixture.process()
	fixture.assert(`SELECT count(*)=5 FROM price_observations`)
}

func (fixture workerFixture) checkLateUpload() {
	identity := fixture.enqueue("submission", 5, func(document map[string]any) {
		observation := firstObservation(document)
		observation["observation_id"] = "00000000-0000-4000-8000-000000000005"
		observation["observed_at"] = "2026-10-26T12:00:00Z"
		observation["product"].(map[string]any)["name"] = "Older listing name"
	})
	fixture.process()
	fixture.assert(`SELECT name='Arroz branco tipo 1 — pacote 5 kg'
        AND metadata_observed_at='2026-10-27T12:00:00Z'::timestamptz FROM shop_products WHERE source_product_id='sku-000123'`)
	fixture.assert(`SELECT count(*)=2 AND bool_and(observed_at='2026-10-26T12:00:00Z'::timestamptz)
        FROM price_observations WHERE payload_id=$1`, identity)
}

func TestWorkerRollsBackConflictsAndInvalidDomainData(test *testing.T) {
	fixture := newWorkerFixture(test)
	fixture.registerShop()
	fixture.enqueue("submission", 1, nil)
	fixture.process()
	for position, failure := range []string{"conflict", "GTIN", "reference"} {
		fixture.checkBadPayload(position+2, failure)
	}
	fixture.enqueue("unavailable", 9, nil)
	fixture.process()
	fixture.assert(`SELECT count(*)=3 FROM price_observations`)
}

func (fixture workerFixture) checkBadPayload(number int, failure string) {
	raw := invalidWorkerPayload(fixture.test, number, failure)
	identity := enqueueWorkerPayload(fixture.test, fixture.context, fixture.database, raw)
	fixture.process()
	fixture.assert(permanentFailureSQL, identity)
	fixture.assert(`SELECT count(*)=2 FROM price_observations`)
	fixture.assert(`SELECT count(*)=1 FROM observation_identities`)
	fixture.assert(`SELECT count(*)=1 FROM shop_products`)
	fixture.assert(`SELECT count(*)=1 FROM products`)
}

func invalidWorkerPayload(test *testing.T, number int, failure string) []byte {
	return workerPayload(test, "submission", number, func(document map[string]any) {
		bad := firstObservation(document)
		good := precedingObservation(test, bad)
		bad["availability"] = "unknown"
		corruptObservation(bad, failure)
		document["observations"] = []any{good, bad}
	})
}

func precedingObservation(test *testing.T, observation map[string]any) map[string]any {
	raw, err := json.Marshal(observation)
	requireNoError(test, err)
	var good map[string]any
	requireNoError(test, json.Unmarshal(raw, &good))
	good["observation_id"] = "00000000-0000-4000-8000-000000000001"
	good["product"].(map[string]any)["source_product_id"] = "brand-new-listing"
	good["availability"] = "in_stock"
	return good
}

func corruptObservation(observation map[string]any, failure string) {
	if failure != "conflict" {
		observation["observation_id"] = "00000000-0000-4000-8000-000000000002"
	}
	switch failure {
	case "GTIN":
		observation["product"].(map[string]any)["gtin"] = "4006381333932"
	case "reference":
		observation["offers"].([]any)[0].(map[string]any)["reference_amount"] = "24.89"
	}
}

func TestWorkerRetriesFailedCommitAndSupportsDeliberateReplay(test *testing.T) {
	fixture := newWorkerFixture(test)
	fixture.registerShop()
	configuration := ingestion.DefaultWorkerConfig()
	configuration.MaxAttempts = 2
	fixture = fixture.configure(configuration)
	fixture.installCommitFailure()
	identity := fixture.enqueue("submission", 1, nil)
	fixture.process()
	fixture.assertPendingAfterFailedCommit(identity)
	fixture.assertNoReadyPayload("retried before backoff elapsed")
	fixture.retryToTerminal(identity)
	fixture.replayAfterRepair(identity)
}

func (fixture workerFixture) installCommitFailure() {
	fixture.execute(`CREATE FUNCTION fail_worker_commit() RETURNS trigger LANGUAGE plpgsql AS $$
        BEGIN RAISE EXCEPTION 'temporary storage failure'; END $$`)
	fixture.execute(`CREATE CONSTRAINT TRIGGER fail_worker_commit AFTER INSERT ON price_observations
        DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION fail_worker_commit()`)
}

func (fixture workerFixture) assertNoDomainRows() {
	fixture.assert(`SELECT count(*)=0 FROM price_observations`)
	fixture.assert(`SELECT count(*)=0 FROM observation_identities`)
	fixture.assert(`SELECT count(*)=0 FROM shop_products`)
	fixture.assert(`SELECT count(*)=0 FROM products`)
}

func (fixture workerFixture) assertPendingAfterFailedCommit(identity int64) {
	fixture.assert(`SELECT status='pending' AND attempts=1 AND next_attempt_at>now()
        AND last_error LIKE '%temporary storage failure%' FROM ingestion_payloads WHERE id=$1`, identity)
	fixture.assertNoDomainRows()
}

func (fixture workerFixture) retryToTerminal(identity int64) {
	fixture.execute(`UPDATE ingestion_payloads SET next_attempt_at=now() WHERE id=$1`, identity)
	fixture.process()
	fixture.assert(`SELECT status='failed' AND attempts=2 AND failed_at IS NOT NULL
        AND payload IS NOT NULL FROM ingestion_payloads WHERE id=$1`, identity)
}

func (fixture workerFixture) replayAfterRepair(identity int64) {
	fixture.execute(`DROP TRIGGER fail_worker_commit ON price_observations`)
	fixture.assertNoReadyPayload("terminal failure retried automatically")
	fixture.execute(`UPDATE ingestion_payloads SET status='pending',failed_at=NULL,
        attempts=0,next_attempt_at=now(),last_error=NULL WHERE id=$1`, identity)
	fixture.process()
	fixture.assert(`SELECT status='processed' AND attempts=1 AND failed_at IS NULL
        AND last_error IS NULL FROM ingestion_payloads WHERE id=$1`, identity)
	fixture.assert(`SELECT count(*)=2 FROM price_observations`)
}

func TestWorkerSkipsLockedAndDeferredRowsAcrossConcurrentWorkers(test *testing.T) {
	fixture := newWorkerFixture(test)
	fixture.registerShop()
	identities := fixture.enqueueDistinct(8)
	fixture.deferSecond(identities)
	lock := fixture.lockPayload(identities[0])
	fixture.process()
	fixture.assert(`SELECT status='processed' FROM ingestion_payloads WHERE id=$1`, identities[2])
	fixture.processConcurrently(4, drainWorker)
	fixture.assert(`SELECT count(*)=6 AND bool_and(attempts=1) FROM ingestion_payloads WHERE status='processed'`)
	fixture.assert(`SELECT count(*)=2 FROM price_observations`)
	fixture.releaseDeferred(lock, identities)
	fixture.enqueueRebatches(4)
	fixture.processConcurrently(4, processRebatch)
	fixture.assert(`SELECT count(*)=12 AND bool_and(status='processed') FROM ingestion_payloads`)
	fixture.assert(`SELECT count(*)=2 FROM price_observations`)
}

func observationNumber(number int) func(map[string]any) {
	return func(document map[string]any) {
		firstObservation(document)["observation_id"] = fmt.Sprintf("00000000-0000-4000-8000-%012d", number)
	}
}

func (fixture workerFixture) enqueueDistinct(count int) []int64 {
	identities := make([]int64, count)
	for position := range identities {
		identities[position] = fixture.enqueue("submission", position+1, observationNumber(position+1))
	}
	return identities
}

func (fixture workerFixture) deferSecond(identities []int64) {
	fixture.execute(`UPDATE ingestion_payloads SET received_at='2026-10-27T12:00:00Z'`)
	fixture.execute(`UPDATE ingestion_payloads SET next_attempt_at=now()+interval '1 hour' WHERE id=$1`, identities[1])
}

func (fixture workerFixture) lockPayload(identity int64) *sql.Tx {
	transaction := fixture.transaction()
	executeSQL(fixture.test, fixture.context, transaction, `SELECT id FROM ingestion_payloads WHERE id=$1 FOR UPDATE`, identity)
	return transaction
}

func drainWorker(requestContext context.Context, worker *ingestion.Worker) error {
	ready := true
	var err error
	for ready && err == nil {
		ready, err = worker.ProcessNext(requestContext)
	}
	return err
}

func processRebatch(requestContext context.Context, worker *ingestion.Worker) error {
	found, err := worker.ProcessNext(requestContext)
	if err == nil && !found {
		return errors.New("rebatch was not selected")
	}
	return err
}

func (fixture workerFixture) processConcurrently(count int, process func(context.Context, *ingestion.Worker) error) {
	results := make(chan error, count)
	var workers sync.WaitGroup
	for range count {
		worker := workerForTest(fixture.test, fixture.database)
		workers.Go(func() { results <- process(fixture.context, worker) })
	}
	workers.Wait()
	close(results)
	for err := range results {
		requireNoError(fixture.test, err)
	}
}

func (fixture workerFixture) releaseDeferred(lock *sql.Tx, identities []int64) {
	requireNoError(fixture.test, lock.Rollback())
	fixture.process()
	fixture.assert(`SELECT status='processed' FROM ingestion_payloads WHERE id=$1`, identities[0])
	fixture.execute(`UPDATE ingestion_payloads SET next_attempt_at=now() WHERE id=$1`, identities[1])
	fixture.process()
	fixture.assert(`SELECT count(*)=8 AND bool_and(status='processed' AND attempts=1) FROM ingestion_payloads`)
	fixture.assert(`SELECT count(*)=2 FROM price_observations`)
}

func (fixture workerFixture) enqueueRebatches(count int) {
	for position := range count {
		fixture.enqueue("submission", position+20, observationNumber(1))
	}
}

func TestWorkerMissingShopRetriesAndRunStartsImmediately(test *testing.T) {
	fixture := newWorkerFixture(test)
	identity := fixture.enqueue("submission", 1, nil)
	configuration := ingestion.DefaultWorkerConfig()
	configuration.PollInterval = time.Hour
	fixture = fixture.configure(configuration)
	fixture.process()
	fixture.assert(`SELECT status='pending' AND attempts=1 AND next_attempt_at>now()
        AND last_error LIKE '%not registered%' FROM ingestion_payloads WHERE id=$1`, identity)
	fixture.registerShop()
	fixture.execute(`UPDATE ingestion_payloads SET next_attempt_at=now() WHERE id=$1`, identity)
	fixture.startWorker()
	fixture.waitForSQL(`SELECT status='processed' FROM ingestion_payloads WHERE id=$1`, nil,
		"Run waited for its one-hour poll interval instead of starting immediately", identity)
	fixture.assert(`SELECT attempts=2 AND last_error IS NULL FROM ingestion_payloads WHERE id=$1`, identity)
	fixture.assert(`SELECT count(*)=2 FROM price_observations`)
}

func (fixture workerFixture) startWorker() {
	requestContext, cancel := context.WithCancel(fixture.context)
	done := make(chan struct{})
	go func() { defer close(done); fixture.worker.Run(requestContext) }()
	fixture.test.Cleanup(func() { stopWorker(fixture.test, cancel, done) })
}

func stopWorker(test *testing.T, cancel context.CancelFunc, done <-chan struct{}) {
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		test.Error("worker did not stop on cancellation")
	}
}

func (fixture workerFixture) queryBoolean(query string, arguments ...any) bool {
	fixture.test.Helper()
	var result bool
	row := fixture.database.QueryRowContext(fixture.context, query, arguments...)
	requireNoError(fixture.test, row.Scan(&result))
	return result
}

func (fixture workerFixture) waitForSQL(query string, done <-chan error, message string, arguments ...any) {
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for !fixture.queryBoolean(query, arguments...) {
		awaitDatabaseEvent(fixture.test, deadline.C, ticker.C, done, message)
	}
}

func awaitDatabaseEvent(test *testing.T, deadline, ticker <-chan time.Time, done <-chan error, message string) {
	select {
	case err := <-done:
		test.Fatalf("worker returned before blocking: %v", err)
	case <-deadline:
		test.Fatal(message)
	case <-ticker:
	}
}

func TestWorkerCancellationRollsBackAndRecovers(test *testing.T) {
	fixture := newWorkerFixture(test)
	fixture.registerShop()
	identity := fixture.enqueue("submission", 1, nil)
	blocker := fixture.blockInserts()
	process := fixture.startSingleProcess()
	fixture.waitForSQL(`SELECT EXISTS (SELECT 1 FROM pg_stat_activity
        WHERE datname=current_database() AND wait_event='advisory')`, process.result, "worker did not reach blocking insert")
	process.cancelAndWait(test)
	fixture.assert(`SELECT status='pending' AND attempts=0 AND last_error IS NULL FROM ingestion_payloads WHERE id=$1`, identity)
	fixture.assertNoDomainRows()
	requireNoError(test, blocker.Rollback())
	fixture.process()
	fixture.assert(`SELECT status='processed' AND attempts=1 FROM ingestion_payloads WHERE id=$1`, identity)
	fixture.assert(`SELECT count(*)=2 FROM price_observations`)
}

func (fixture workerFixture) blockInserts() *sql.Tx {
	blocker := fixture.transaction()
	executeSQL(fixture.test, fixture.context, blocker, `SELECT pg_advisory_xact_lock(81234)`)
	fixture.execute(`CREATE FUNCTION block_worker() RETURNS trigger LANGUAGE plpgsql AS $$
        BEGIN PERFORM pg_advisory_xact_lock(81234); RETURN NEW; END $$`)
	fixture.execute(`CREATE TRIGGER block_worker BEFORE INSERT ON price_observations
        FOR EACH ROW EXECUTE FUNCTION block_worker()`)
	return blocker
}

type runningProcess struct {
	cancel   context.CancelFunc
	result   <-chan error
	finished <-chan struct{}
}

func (fixture workerFixture) startSingleProcess() runningProcess {
	requestContext, cancel := context.WithCancel(fixture.context)
	result, finished := make(chan error, 1), make(chan struct{})
	go processOnce(requestContext, fixture.worker, result, finished)
	process := runningProcess{cancel, result, finished}
	fixture.test.Cleanup(func() { stopWorker(fixture.test, cancel, finished) })
	return process
}

func processOnce(requestContext context.Context, worker *ingestion.Worker, result chan<- error, finished chan struct{}) {
	defer close(finished)
	_, err := worker.ProcessNext(requestContext)
	result <- err
}

func (process runningProcess) cancelAndWait(test *testing.T) {
	process.cancel()
	err := process.awaitResult(test)
	if !errors.Is(err, context.Canceled) {
		test.Fatalf("canceled worker returned %v", err)
	}
}

func (process runningProcess) awaitResult(test *testing.T) error {
	select {
	case err := <-process.result:
		return err
	case <-time.After(5 * time.Second):
		test.Fatal("worker did not stop after cancellation")
	}
	return nil
}

func TestWorkerSkipsUnchangedSameDayHistoryAndRetainsIdentities(test *testing.T) {
	fixture := newWorkerFixture(test)
	fixture.registerShop()
	fixture.freshMeasurement(1, "2026-10-27T12:00:00Z", nil)
	fixture.freshMeasurement(2, "2026-10-27T13:00:00Z", changedSourceURL)
	fixture.assert(`SELECT count(*)=2 AND bool_and(observed_at='2026-10-27T12:00:00Z'
        AND last_seen_at='2026-10-27T13:00:00Z') FROM price_observations`)
	fixture.assert(`SELECT count(*)=2 FROM observation_identities`)
	fixture.assert(`SELECT count(*)=2 AND bool_and(status='processed') FROM ingestion_payloads`)
	fixture.execute(`UPDATE ingestion_payloads SET payload=NULL,payload_removed_at=now()`)
	fixture.freshMeasurement(3, "2026-10-27T14:00:00Z", nil)
	fixture.assert(`SELECT count(*)=2 AND bool_and(last_seen_at='2026-10-27T14:00:00Z') FROM price_observations`)
	fixture.assert(`SELECT count(*)=3 FROM observation_identities`)
}

func (fixture workerFixture) freshMeasurement(number int, observedAt string, change func(map[string]any)) {
	fixture.enqueue("submission", number, func(payload map[string]any) {
		observation := firstObservation(payload)
		observation["observation_id"] = fmt.Sprintf("00000000-0000-4000-8000-%012d", number)
		observation["observed_at"] = observedAt
		if change != nil {
			change(observation)
		}
	})
	fixture.process()
}

func changedSourceURL(observation map[string]any) {
	observation["source_url"] = "https://mercado.example.com/api/search?page=2"
}

func TestWorkerKeepsSameDayPriceChangesIncludingReturnToEarlierPrice(test *testing.T) {
	fixture := newWorkerFixture(test)
	fixture.registerShop()
	fixture.freshMeasurement(1, "2026-10-27T12:00:00Z", nil)
	fixture.freshMeasurement(2, "2026-10-27T13:00:00Z", increasedRegularPrice)
	fixture.freshMeasurement(3, "2026-10-27T14:00:00Z", nil)
	fixture.freshMeasurement(4, "2026-10-27T15:00:00Z", nil)
	fixture.assert(`SELECT count(*)=6 AND count(DISTINCT observation_id)=3 FROM price_observations`)
	fixture.assert(`SELECT count(*)=4 FROM observation_identities`)
	fixture.assert(`SELECT count(*)=2 AND bool_and(last_seen_at='2026-10-27T15:00:00Z')
        FROM price_observations WHERE observed_at='2026-10-27T14:00:00Z'`)
}

func increasedRegularPrice(observation map[string]any) {
	regularOffer(observation)["amount"] = "25.90"
}

func regularOffer(observation map[string]any) map[string]any {
	return observation["offers"].([]any)[0].(map[string]any)
}

func TestWorkerSameDayDeduplicationUsesSaoPauloCalendar(test *testing.T) {
	fixture := newWorkerFixture(test)
	fixture.registerShop()
	fixture.freshMeasurement(1, "2026-10-27T23:00:00Z", nil)
	fixture.freshMeasurement(2, "2026-10-28T02:59:59Z", nil)
	fixture.assert(`SELECT count(*)=2 FROM price_observations`)
	fixture.freshMeasurement(3, "2026-10-28T03:00:00Z", nil)
	fixture.assert(`SELECT count(*)=4 FROM price_observations`)
}

func TestWorkerOfferOrderDoesNotCreateDuplicateDailyHistory(test *testing.T) {
	fixture := newWorkerFixture(test)
	fixture.registerShop()
	fixture.freshMeasurement(1, "2026-10-27T12:00:00Z", nil)
	fixture.freshMeasurement(2, "2026-10-27T13:00:00Z", reverseOffers)
	fixture.assert(`SELECT count(*)=2 FROM price_observations`)
	fixture.assert(regularPriceSQL)
	fixture.assert(memberPriceSQL)
}

func reverseOffers(observation map[string]any) {
	offers := observation["offers"].([]any)
	offers[0], offers[1] = offers[1], offers[0]
}

func TestWorkerDailyDeduplicationPreservesDistinctOfferStates(test *testing.T) {
	for _, change := range []string{"amount", "zero", "reference", "member-price", "payment", "membership", "coupon", "minimum", "description", "package", "availability", "empty-offers", "removed-offer", "seller", "store", "postal-code", "city", "state", "channel", "unknown-store"} {
		test.Run(change, func(test *testing.T) { checkDistinctDailyState(test, change) })
	}
}

func checkDistinctDailyState(test *testing.T, change string) {
	fixture := newWorkerFixture(test)
	fixture.registerShop()
	fixture.freshMeasurement(1, "2026-10-27T12:00:00Z", nil)
	fixture.freshMeasurement(2, "2026-10-27T13:00:00Z", func(observation map[string]any) {
		changeMeasurementState(observation, change)
	})
	fixture.assert(`SELECT count(DISTINCT observation_id)=2 FROM price_observations`)
	fixture.assert(`SELECT count(*)=2 AND bool_and(status='processed') FROM ingestion_payloads`)
}

func changeMeasurementState(observation map[string]any, change string) {
	if changeOfferState(observation, change) {
		return
	}
	if changeContextState(observation, change) {
		return
	}
	if change == "package" {
		product := observation["product"].(map[string]any)
		product["package"].(map[string]any)["quantity"] = "6"
		return
	}
	observation["availability"] = "unknown"
}

func changeOfferState(observation map[string]any, change string) bool {
	offer := regularOffer(observation)
	switch change {
	case "amount":
		offer["amount"] = "25.90"
	case "zero":
		offer["amount"] = "0.00"
	case "reference":
		offer["reference_amount"] = nil
	case "member-price":
		observation["offers"].([]any)[1].(map[string]any)["amount"] = "23.90"
	case "empty-offers":
		observation["offers"] = []any{}
	case "removed-offer":
		observation["offers"] = observation["offers"].([]any)[:1]
	default:
		return changeOfferConditions(observation, change)
	}
	return true
}

func changeOfferConditions(observation map[string]any, change string) bool {
	fields := map[string]string{"payment": "payment_method", "membership": "membership_required",
		"coupon": "coupon_required", "minimum": "minimum_quantity", "description": "description"}
	values := map[string]any{"payment": "cash", "membership": nil, "coupon": true,
		"minimum": "2", "description": "Different price conditions"}
	field, exists := fields[change]
	if !exists {
		return false
	}
	offer := regularOffer(observation)
	conditions := offer["conditions"].(map[string]any)
	conditions[field] = values[change]
	return true
}

func changeContextState(observation map[string]any, change string) bool {
	fields := map[string]string{"seller": "seller_id", "store": "store_id", "postal-code": "postal_code",
		"city": "city", "state": "state", "channel": "channel", "unknown-store": "store_id"}
	values := map[string]any{"seller": "other-seller", "store": "other-store", "postal-code": "01310200",
		"city": "Another city", "state": "PR", "channel": "pickup", "unknown-store": nil}
	field, exists := fields[change]
	if !exists {
		return false
	}
	location := observation["context"].(map[string]any)
	location[field] = values[change]
	return true
}

func TestWorkerOrdersChangesWithinOnePayloadBySourceTime(test *testing.T) {
	fixture := newWorkerFixture(test)
	fixture.registerShop()
	fixture.enqueue("submission", 1, func(payload map[string]any) {
		original := firstObservation(payload)
		payload["observations"] = []any{
			variantObservation(test, original, 3, "2026-10-27T12:00:00.2Z", "24.90"),
			variantObservation(test, original, 1, "2026-10-27T12:00:00Z", "24.90"),
			variantObservation(test, original, 2, "2026-10-27T12:00:00.1Z", "25.90"),
		}
	})
	fixture.process()
	fixture.assert(`SELECT count(*)=6 AND count(DISTINCT observation_id)=3 FROM price_observations`)
}

func variantObservation(test *testing.T, original map[string]any, number int, observedAt, amount string) map[string]any {
	raw, err := json.Marshal(original)
	requireNoError(test, err)
	var observation map[string]any
	requireNoError(test, json.Unmarshal(raw, &observation))
	observation["observation_id"] = fmt.Sprintf("00000000-0000-4000-8000-%012d", number)
	observation["observed_at"] = observedAt
	regularOffer(observation)["amount"] = amount
	return observation
}

func TestWorkerConcurrentUnchangedMeasurementsStoreHistoryOnce(test *testing.T) {
	fixture := newWorkerFixture(test)
	fixture.registerShop()
	fixture.enqueueDistinct(4)
	fixture.processConcurrently(4, drainWorker)
	fixture.assert(`SELECT count(*)=2 FROM price_observations`)
	fixture.assert(`SELECT count(*)=4 FROM observation_identities`)
	fixture.assert(`SELECT count(*)=4 AND bool_and(status='processed') FROM ingestion_payloads`)
}

func TestWorkerUnchangedConfirmationsRemainLatestAfterLateChangedUpload(test *testing.T) {
	fixture := newWorkerFixture(test)
	fixture.registerShop()
	fixture.freshMeasurement(1, "2026-10-27T12:00:00Z", nil)
	requireNoError(test, dailysummary.Refresh(fixture.context, fixture.database))
	fixture.freshMeasurement(2, "2026-10-27T14:00:00Z", nil)
	fixture.assert(`SELECT bool_and(generation>summarized_generation) FROM history_daily_days`)
	fixture.freshMeasurement(3, "2026-10-27T13:00:00Z", increasedRegularPrice)
	fixture.assert(`SELECT count(*)=4 FROM price_observations`)
	fixture.assertLatestConfirmedPrice("1")
	fixture.assertLatestConfirmedPrice("global")
	requireNoError(test, dailysummary.Refresh(fixture.context, fixture.database))
	fixture.assertLatestConfirmedPrice("global")
}

func (fixture workerFixture) assertLatestConfirmedPrice(product string) {
	store := webapi.PostgresStore{DB: fixture.database}
	request := webapi.HistoryRequest{Product: product, Start: "2026-10-27", End: "2026-10-27"}
	result, err := store.History(fixture.context, request)
	requireNoError(fixture.test, err)
	if result.CollectedAt == nil || result.CollectedAt.Format(time.RFC3339) != "2026-10-27T14:00:00Z" {
		fixture.test.Fatalf("lost latest unchanged confirmation: %v", result.CollectedAt)
	}
	if product != "global" && (result.LatestPriceMinor == nil || *result.LatestPriceMinor != 2490) {
		fixture.test.Fatalf("late changed upload replaced later confirmation: %v", result.LatestPriceMinor)
	}
}

func TestWorkerRepeatedUnavailableMeasurementsRemainOneUnknownPrice(test *testing.T) {
	fixture := newWorkerFixture(test)
	fixture.registerShop()
	fixture.freshMeasurement(1, "2026-10-27T12:00:00Z", unavailableWithoutOffers)
	fixture.freshMeasurement(2, "2026-10-27T13:00:00Z", unavailableWithoutOffers)
	fixture.assert(`SELECT count(*)=1 AND bool_and(price_amount IS NULL AND offer_count=0
        AND availability='out_of_stock' AND last_seen_at='2026-10-27T13:00:00Z') FROM price_observations`)
}

func unavailableWithoutOffers(observation map[string]any) {
	observation["availability"], observation["offers"] = "out_of_stock", []any{}
}

func TestWorkerRollsBackUnchangedConfirmationWhenLaterObservationFails(test *testing.T) {
	fixture := newWorkerFixture(test)
	fixture.registerShop()
	fixture.freshMeasurement(1, "2026-10-27T12:00:00Z", nil)
	identity := fixture.enqueue("submission", 2, func(payload map[string]any) {
		unchanged := variantObservation(test, firstObservation(payload), 2, "2026-10-27T13:00:00Z", "24.90")
		invalid := variantObservation(test, firstObservation(payload), 3, "2026-10-27T14:00:00Z", "24.90")
		invalid["product"].(map[string]any)["gtin"] = "4006381333932"
		payload["observations"] = []any{unchanged, invalid}
	})
	fixture.process()
	fixture.assert(permanentFailureSQL, identity)
	fixture.assert(`SELECT count(*)=2 AND bool_and(last_seen_at IS NULL) FROM price_observations`)
	fixture.assert(`SELECT count(*)=1 FROM observation_identities`)
}

func TestWorkerCompactedIdentityStillRejectsConflictsAfterPayloadCleanup(test *testing.T) {
	fixture := newWorkerFixture(test)
	fixture.registerShop()
	fixture.freshMeasurement(1, "2026-10-27T12:00:00Z", nil)
	fixture.freshMeasurement(2, "2026-10-27T13:00:00Z", nil)
	fixture.execute(`UPDATE ingestion_payloads SET payload=NULL,payload_removed_at=now()`)
	identity := fixture.enqueue("submission", 3, conflictingConfirmation)
	fixture.process()
	fixture.assert(permanentFailureSQL, identity)
	fixture.enqueue("submission", 4, unchangedConfirmation)
	fixture.process()
	fixture.assert(`SELECT count(*)=2 AND bool_and(last_seen_at='2026-10-27T13:00:00Z') FROM price_observations`)
	fixture.assert(`SELECT count(*)=2 FROM observation_identities`)
	fixture.assert(`SELECT status='processed' FROM ingestion_payloads WHERE submission_id='00000000-0000-4000-8000-000000000004'`)
}

func unchangedConfirmation(payload map[string]any) {
	observation := firstObservation(payload)
	observation["observation_id"] = "00000000-0000-4000-8000-000000000002"
	observation["observed_at"] = "2026-10-27T13:00:00Z"
}

func conflictingConfirmation(payload map[string]any) {
	unchangedConfirmation(payload)
	increasedRegularPrice(firstObservation(payload))
}
