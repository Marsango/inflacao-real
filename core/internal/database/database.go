package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// ConnectionString uses DATABASE_URL when set, otherwise the POSTGRES_* settings
// shared with Compose. Credentials are escaped instead of interpolated into SQL.
func ConnectionString(getenv func(string) string) (string, error) {
	if value := getenv("DATABASE_URL"); value != "" {
		if _, err := pgx.ParseConfig(value); err != nil {
			return "", errors.New("invalid DATABASE_URL")
		}
		return value, nil
	}
	valueOr := func(name, fallback string) string {
		if value := getenv(name); value != "" {
			return value
		}
		return fallback
	}
	password := getenv("POSTGRES_PASSWORD")
	if password == "" {
		return "", errors.New("set DATABASE_URL or POSTGRES_PASSWORD")
	}
	port := valueOr("POSTGRES_PORT", "5432")
	if number, err := strconv.ParseUint(port, 10, 16); err != nil || number == 0 {
		return "", errors.New("POSTGRES_PORT must be between 1 and 65535")
	}
	connectionURL := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(valueOr("POSTGRES_USER", "inflacao_real"), password),
		Host:   net.JoinHostPort(valueOr("POSTGRES_HOST", "127.0.0.1"), port),
		Path:   "/" + valueOr("POSTGRES_DB", "inflacao_real"),
		RawQuery: url.Values{
			"sslmode": {valueOr("POSTGRES_SSLMODE", "disable")},
		}.Encode(),
	}
	if _, err := pgx.ParseConfig(connectionURL.String()); err != nil {
		return "", errors.New("invalid PostgreSQL connection settings")
	}
	return connectionURL.String(), nil
}

// Open returns a checked connection pool. The caller owns the pool.
func Open(ctx context.Context, connectionString string) (*sql.DB, error) {
	config, err := pgx.ParseConfig(connectionString)
	if err != nil {
		return nil, errors.New("invalid PostgreSQL connection settings")
	}
	config.RuntimeParams["timezone"] = "UTC"
	db := stdlib.OpenDB(*config)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	return db, nil
}
