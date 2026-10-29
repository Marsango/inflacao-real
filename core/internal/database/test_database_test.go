package database

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// testDatabase gives each integration test a fresh, migrated PostgreSQL instance.
// Cleanup runs even after t.Fatal: close the pool before removing the container.
func testDatabase(t *testing.T) (context.Context, *sql.DB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	t.Cleanup(cancel)

	container, err := postgres.Run(ctx, "postgres:18.6",
		postgres.WithDatabase("inflacao_real_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	if container != nil {
		t.Cleanup(func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cleanupCancel()
			if err := container.Terminate(cleanupCtx); err != nil {
				t.Errorf("terminate PostgreSQL testcontainer: %v", err)
			}
		})
	}
	if err != nil {
		t.Fatalf("start PostgreSQL testcontainer: %v", err)
	}
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get test PostgreSQL connection string: %v", err)
	}
	db, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close test PostgreSQL pool: %v", err)
		}
	})
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	return ctx, db
}
