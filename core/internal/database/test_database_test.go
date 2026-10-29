package database

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// Each test owns a fresh migrated database. Cleanup closes its pool first.
func testDatabase(test *testing.T) (context.Context, *sql.DB) {
	test.Helper()
	requestContext, cancel := context.WithTimeout(test.Context(), 2*time.Minute)
	test.Cleanup(cancel)
	container := startPostgres(test, requestContext)
	pool := openTestPool(test, requestContext, container)
	requireNoError(test, Migrate(requestContext, pool))
	return requestContext, pool
}

func startPostgres(test *testing.T, requestContext context.Context) *postgres.PostgresContainer {
	test.Helper()
	container, err := postgres.Run(requestContext, "postgres:18.6", postgres.WithDatabase("inflacao_real_test"),
		postgres.WithUsername("test"), postgres.WithPassword("test"), postgres.BasicWaitStrategies())
	registerContainerCleanup(test, container)
	if err != nil {
		test.Fatalf("start PostgreSQL testcontainer: %v", err)
	}
	return container
}

func registerContainerCleanup(test *testing.T, container *postgres.PostgresContainer) {
	test.Helper()
	if container == nil {
		return
	}
	test.Cleanup(func() { terminatePostgres(test, container) })
}

func terminatePostgres(test *testing.T, container *postgres.PostgresContainer) {
	requestContext, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := container.Terminate(requestContext); err != nil {
		test.Errorf("terminate PostgreSQL testcontainer: %v", err)
	}
}

func openTestPool(test *testing.T, requestContext context.Context, container *postgres.PostgresContainer) *sql.DB {
	test.Helper()
	connectionString, err := container.ConnectionString(requestContext, "sslmode=disable")
	if err != nil {
		test.Fatalf("get test PostgreSQL connection string: %v", err)
	}
	pool, err := Open(requestContext, connectionString)
	requireNoError(test, err)
	test.Cleanup(func() { closeTestPool(test, pool) })
	return pool
}

func closeTestPool(test *testing.T, pool *sql.DB) {
	if err := pool.Close(); err != nil {
		test.Errorf("close test PostgreSQL pool: %v", err)
	}
}

func requireNoError(test *testing.T, err error) {
	test.Helper()
	if err != nil {
		test.Fatal(err)
	}
}

type sqlExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func executeSQL(test *testing.T, requestContext context.Context, executor sqlExecutor, query string, arguments ...any) {
	test.Helper()
	_, err := executor.ExecContext(requestContext, query, arguments...)
	requireNoError(test, err)
}
