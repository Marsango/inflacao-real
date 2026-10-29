package database

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestConnectionStringPreservesCredentials(t *testing.T) {
	environment := map[string]string{
		"POSTGRES_USER":     "collector@local",
		"POSTGRES_PASSWORD": "p@ss:/?# with spaces",
		"POSTGRES_HOST":     "::1",
		"POSTGRES_PORT":     "55432",
	}
	dsn, err := ConnectionString(func(name string) string { return environment[name] })
	if err != nil {
		t.Fatal(err)
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal("generated connection settings are invalid")
	}
	if config.User != environment["POSTGRES_USER"] || config.Password != environment["POSTGRES_PASSWORD"] {
		t.Fatal("connection settings changed credentials")
	}
	if config.Host != "::1" || config.Port != 55432 || config.Database != "inflacao_real" {
		t.Fatal("connection settings lost host, port, or database")
	}
}

func TestConnectionStringRejectsMissingPasswordAndBadPorts(t *testing.T) {
	for _, port := range []string{"0", "65536", "abc"} {
		t.Run(port, func(t *testing.T) {
			_, err := ConnectionString(func(name string) string {
				switch name {
				case "POSTGRES_PASSWORD":
					return "test-only-password"
				case "POSTGRES_PORT":
					return port
				}
				return ""
			})
			if err == nil {
				t.Fatal("expected invalid port to be rejected")
			}
		})
	}
	if _, err := ConnectionString(func(string) string { return "" }); err == nil {
		t.Fatal("expected missing credentials to be rejected")
	}
}

func TestInvalidDatabaseURLDoesNotExposeCredentials(t *testing.T) {
	_, err := ConnectionString(func(name string) string {
		if name == "DATABASE_URL" {
			return "postgres://user:secret-value@localhost:invalid/db"
		}
		return ""
	})
	if err == nil || strings.Contains(err.Error(), "secret-value") {
		t.Fatal("invalid URL must fail without exposing credentials")
	}
}
