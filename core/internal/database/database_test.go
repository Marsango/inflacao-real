package database

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestConnectionStringPreservesCredentials(test *testing.T) {
	environment := map[string]string{
		"POSTGRES_USER":     "collector@local",
		"POSTGRES_PASSWORD": "p@ss:/?# with spaces",
		"POSTGRES_HOST":     "::1",
		"POSTGRES_PORT":     "55432",
	}
	connectionString, err := ConnectionString(func(name string) string { return environment[name] })
	if err != nil {
		test.Fatal(err)
	}
	assertConnectionSettings(test, connectionString, environment)
}

func assertConnectionSettings(test *testing.T, connectionString string, environment map[string]string) {
	config, err := pgx.ParseConfig(connectionString)
	if err != nil {
		test.Fatal("generated connection settings are invalid")
	}
	if config.User != environment["POSTGRES_USER"] || config.Password != environment["POSTGRES_PASSWORD"] {
		test.Fatal("connection settings changed credentials")
	}
	if config.Host != "::1" || config.Port != 55432 || config.Database != "inflacao_real" {
		test.Fatal("connection settings lost host, port, or database")
	}
}

func TestConnectionStringRejectsMissingPasswordAndBadPorts(test *testing.T) {
	for _, port := range []string{"0", "65536", "abc"} {
		test.Run(port, func(test *testing.T) { assertInvalidPort(test, port) })
	}
	if _, err := ConnectionString(func(string) string { return "" }); err == nil {
		test.Fatal("expected missing credentials to be rejected")
	}
}

func assertInvalidPort(test *testing.T, port string) {
	environment := map[string]string{"POSTGRES_PASSWORD": "test-only-password", "POSTGRES_PORT": port}
	_, err := ConnectionString(func(name string) string { return environment[name] })
	if err == nil {
		test.Fatal("expected invalid port to be rejected")
	}
}

func TestInvalidDatabaseURLDoesNotExposeCredentials(test *testing.T) {
	_, err := ConnectionString(func(name string) string {
		if name == "DATABASE_URL" {
			return "postgres://user:secret-value@localhost:invalid/db"
		}
		return ""
	})
	if err == nil || strings.Contains(err.Error(), "secret-value") {
		test.Fatal("invalid URL must fail without exposing credentials")
	}
}
