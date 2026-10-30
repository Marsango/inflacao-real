package database

import (
	"context"
	"database/sql"
	"testing"

	"inflacao-real/core/internal/testdatabase"
)

func testDatabase(test *testing.T) (context.Context, *sql.DB) {
	test.Helper()
	return testdatabase.New(test, Open, Migrate)
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
