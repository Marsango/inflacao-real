package database

import (
	"context"
	"database/sql"
	"testing"

	"inflacao-real/core/internal/ingestion"
)

type workerFixture struct {
	test     *testing.T
	context  context.Context
	database *sql.DB
	worker   *ingestion.Worker
}

func newWorkerFixture(test *testing.T) workerFixture {
	test.Helper()
	requestContext, pool := testDatabase(test)
	return workerFixture{test, requestContext, pool, workerForTest(test, pool)}
}

func (fixture workerFixture) registerShop() {
	registerWorkerShop(fixture.test, fixture.context, fixture.database)
}

func (fixture workerFixture) assert(query string, arguments ...any) {
	fixture.test.Helper()
	assertWorkerSQL(fixture.test, fixture.context, fixture.database, query, arguments...)
}

func (fixture workerFixture) execute(query string, arguments ...any) {
	fixture.test.Helper()
	executeSQL(fixture.test, fixture.context, fixture.database, query, arguments...)
}

func (fixture workerFixture) enqueue(example string, number int, change func(map[string]any)) int64 {
	fixture.test.Helper()
	raw := workerPayload(fixture.test, example, number, change)
	return enqueueWorkerPayload(fixture.test, fixture.context, fixture.database, raw)
}

func (fixture workerFixture) process() {
	fixture.test.Helper()
	processWorkerPayload(fixture.test, fixture.context, fixture.worker)
}

func (fixture workerFixture) queryIdentity(query string, arguments ...any) int64 {
	fixture.test.Helper()
	row := fixture.database.QueryRowContext(fixture.context, query, arguments...)
	var identity int64
	requireNoError(fixture.test, row.Scan(&identity))
	return identity
}

func (fixture workerFixture) transaction() *sql.Tx {
	fixture.test.Helper()
	transaction, err := fixture.database.BeginTx(fixture.context, nil)
	requireNoError(fixture.test, err)
	fixture.test.Cleanup(func() { transaction.Rollback() })
	return transaction
}

func (fixture workerFixture) assertNoReadyPayload(message string) {
	fixture.test.Helper()
	found, err := fixture.worker.ProcessNext(fixture.context)
	if found || err != nil {
		fixture.test.Fatalf("%s: %t, %v", message, found, err)
	}
}

func (fixture workerFixture) configure(configuration ingestion.WorkerConfig) workerFixture {
	fixture.test.Helper()
	worker, err := ingestion.NewWorker(fixture.database, configuration)
	requireNoError(fixture.test, err)
	fixture.worker = worker
	return fixture
}
