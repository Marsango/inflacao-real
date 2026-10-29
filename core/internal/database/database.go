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

func ConnectionString(environment func(string) string) (string, error) {
	if value := environment("DATABASE_URL"); value != "" {
		return validateConnectionString(value, "invalid DATABASE_URL")
	}
	if environment("POSTGRES_PASSWORD") == "" {
		return "", errors.New("set DATABASE_URL or POSTGRES_PASSWORD")
	}
	port := environmentOr(environment, "POSTGRES_PORT", "5432")
	if number, err := strconv.ParseUint(port, 10, 16); err != nil || number == 0 {
		return "", errors.New("POSTGRES_PORT must be between 1 and 65535")
	}
	connectionURL := postgresURL(environment, port)
	return validateConnectionString(connectionURL.String(), "invalid PostgreSQL connection settings")
}

func validateConnectionString(value, message string) (string, error) {
	if _, err := pgx.ParseConfig(value); err != nil {
		return "", errors.New(message)
	}
	return value, nil
}

func environmentOr(environment func(string) string, name, fallback string) string {
	if value := environment(name); value != "" {
		return value
	}
	return fallback
}

func postgresURL(environment func(string) string, port string) url.URL {
	username := environmentOr(environment, "POSTGRES_USER", "inflacao_real")
	host := environmentOr(environment, "POSTGRES_HOST", "127.0.0.1")
	database := environmentOr(environment, "POSTGRES_DB", "inflacao_real")
	options := url.Values{"sslmode": {environmentOr(environment, "POSTGRES_SSLMODE", "disable")}}
	return url.URL{Scheme: "postgres", User: url.UserPassword(username, environment("POSTGRES_PASSWORD")),
		Host: net.JoinHostPort(host, port), Path: "/" + database, RawQuery: options.Encode()}
}

// Open returns a checked connection pool. The caller owns the pool.
func Open(requestContext context.Context, connectionString string) (*sql.DB, error) {
	configuration, err := pgx.ParseConfig(connectionString)
	if err != nil {
		return nil, errors.New("invalid PostgreSQL connection settings")
	}
	configuration.RuntimeParams["timezone"] = "UTC"
	pool := stdlib.OpenDB(*configuration)
	configurePool(pool)
	if err := pool.PingContext(requestContext); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	return pool, nil
}

func configurePool(pool *sql.DB) {
	pool.SetMaxOpenConns(10)
	pool.SetMaxIdleConns(5)
	pool.SetConnMaxLifetime(30 * time.Minute)
}
