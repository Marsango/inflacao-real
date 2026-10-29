package monitoring_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"inflacao-real/core/internal/ingestion"
	"inflacao-real/core/internal/monitoring"
)

type storeFunc func(context.Context) ([]monitoring.Target, error)

func (function storeFunc) List(requestContext context.Context) ([]monitoring.Target, error) {
	return function(requestContext)
}

func scopeRequest(t *testing.T, store monitoring.Store, token string) *httptest.ResponseRecorder {
	t.Helper()
	auth := ingestion.Auth{Token: "secret", ScraperID: "test", ShopIDs: []string{"shop"}}
	router, err := ingestion.NewRouter(nil, auth)
	if err != nil {
		t.Fatal(err)
	}
	monitoring.RegisterRoutes(router, store, auth.Authenticate)
	request := httptest.NewRequest("GET", "/api/v1/monitoring-targets", nil)
	request.Header.Set("Authorization", token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestScopeRequiresAuthentication(t *testing.T) {
	store := storeFunc(func(context.Context) ([]monitoring.Target, error) {
		t.Fatal("unauthenticated request reached storage")
		return nil, nil
	})
	response := scopeRequest(t, store, "Bearer wrong")
	if response.Code != 401 {
		t.Fatalf("expected unauthorized, got %d", response.Code)
	}
}

func TestScopeReturnsVersionedEmptyList(t *testing.T) {
	store := storeFunc(func(requestContext context.Context) ([]monitoring.Target, error) {
		if _, bounded := requestContext.Deadline(); !bounded {
			t.Error("missing database timeout")
		}
		return []monitoring.Target{}, nil
	})
	response := scopeRequest(t, store, "Bearer secret")
	if response.Code != 200 || response.Body.String() != `{"schema_version":"1.0","targets":[]}` {
		t.Fatalf("unexpected scope: %d %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("scope must not be cached")
	}
}

func TestScopeStorageFailureDoesNotExposeDetails(t *testing.T) {
	store := storeFunc(func(context.Context) ([]monitoring.Target, error) {
		return nil, errors.New("private database details")
	})
	response := scopeRequest(t, store, "Bearer secret")
	if response.Code != 500 || strings.Contains(response.Body.String(), "private") {
		t.Fatalf("unexpected failure: %d %s", response.Code, response.Body.String())
	}
}
