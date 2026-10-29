package ingestion

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

type storeFunc func(context.Context, Submission) (bool, error)

func (f storeFunc) Enqueue(ctx context.Context, submission Submission) (bool, error) {
	return f(ctx, submission)
}

func example(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile("../../../docs/contracts/ingestion/v1/examples/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func testAuth() Auth {
	return Auth{Token: "test-secret", ScraperID: "collector-sp-01", ShopIDs: []string{"mercado-exemplo"}}
}

func request(handler http.Handler, body []byte, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingestion/submissions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-secret")
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

func TestEmbeddedSchemaMatchesContract(t *testing.T) {
	want, err := os.ReadFile("../../../docs/contracts/ingestion/v1/schema.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(schemaJSON, want) {
		t.Fatal("runtime schema differs from contract; run go generate ./internal/ingestion")
	}
}

func TestSubmissionResponses(t *testing.T) {
	for _, tc := range []struct {
		name     string
		inserted bool
		err      error
		status   int
	}{
		{"new", true, nil, http.StatusAccepted},
		{"duplicate", false, nil, http.StatusOK},
		{"database failure", false, errors.New("private database detail"), http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := example(t, "submission")
			calls := 0
			router, err := NewRouter(storeFunc(func(ctx context.Context, s Submission) (bool, error) {
				calls++
				if s.ScraperID != "collector-sp-01" || s.ShopID != "mercado-exemplo" || s.SchemaVersion != "1.0" || s.SubmissionID == "" || s.CreatedAt == "" || !bytes.Equal(s.Payload, body) {
					t.Errorf("metadata or original payload was lost: %+v", s)
				}
				if _, ok := ctx.Deadline(); !ok {
					t.Error("database context must have a deadline")
				}
				return tc.inserted, tc.err
			}), testAuth())
			if err != nil {
				t.Fatal(err)
			}
			response := request(router, body, nil)
			if response.Code != tc.status || calls != 1 {
				t.Fatalf("status %d, store calls %d; want %d and 1", response.Code, calls, tc.status)
			}
			if tc.err == nil && response.Body.Len() != 0 {
				t.Fatal("success must have an empty body")
			}
			if tc.err != nil && response.Body.String() != "{\"error\":\"ingestion_unavailable\"}" {
				t.Fatal("database errors must not expose internal details")
			}
		})
	}
}

func TestRejectsInvalidRequestsBeforeStorage(t *testing.T) {
	body := example(t, "submission")
	mutate := func(change func(map[string]any)) []byte {
		var doc map[string]any
		if err := json.Unmarshal(body, &doc); err != nil {
			t.Fatal(err)
		}
		change(doc)
		result, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	observation := func(doc map[string]any) map[string]any { return doc["observations"].([]any)[0].(map[string]any) }
	for _, tc := range []struct {
		name    string
		body    []byte
		headers map[string]string
		status  int
	}{
		{"missing token", body, map[string]string{"Authorization": ""}, 401},
		{"wrong token", body, map[string]string{"Authorization": "Bearer wrong"}, 401},
		{"content type", body, map[string]string{"Content-Type": "text/plain"}, 415},
		{"charset", body, map[string]string{"Content-Type": "application/json; charset=latin1"}, 415},
		{"compressed", body, map[string]string{"Content-Encoding": "gzip"}, 415},
		{"too large", append(bytes.Repeat([]byte(" "), MaxBodyBytes), body...), nil, 413},
		{"empty", nil, nil, 400},
		{"malformed", []byte(`{"schema_version":`), nil, 400},
		{"trailing document", append(bytes.Clone(body), []byte("{}")...), nil, 400},
		{"duplicate key", bytes.Replace(body, []byte(`"schema_version": "1.0"`), []byte(`"schema_version":"1.0","schema_version":"1.0"`), 1), nil, 400},
		{"escaped duplicate key", bytes.Replace(body, []byte(`"schema_version": "1.0"`), []byte(`"schema_version":"1.0","schema_versio\u006e":"1.0"`), 1), nil, 400},
		{"unpaired surrogate", bytes.Replace(body, []byte(`"schema_version": "1.0"`), []byte(`"schema_version":"\ud800"`), 1), nil, 400},
		{"invalid UTF8", append(bytes.Clone(body), 0xff), nil, 400},
		{"unknown field", mutate(func(d map[string]any) { d["status"] = "processed" }), nil, 400},
		{"unknown version", mutate(func(d map[string]any) { d["schema_version"] = "2.0" }), nil, 400},
		{"missing required", mutate(func(d map[string]any) { delete(d, "run_id") }), nil, 400},
		{"different scraper", mutate(func(d map[string]any) { d["scraper_id"] = "other-scraper" }), nil, 403},
		{"different shop", mutate(func(d map[string]any) { d["shop_id"] = "other-shop" }), nil, 403},
		{"invalid calendar date", mutate(func(d map[string]any) { d["created_at"] = "2026-02-30T12:00:00Z" }), nil, 400},
		{"non UTC timestamp", mutate(func(d map[string]any) { d["created_at"] = "2026-10-27T09:00:00-03:00" }), nil, 400},
		{"empty observations", mutate(func(d map[string]any) { d["observations"] = []any{} }), nil, 400},
		{"too many observations", mutate(func(d map[string]any) {
			items := make([]any, 101)
			for i := range items {
				items[i] = observation(d)
			}
			d["observations"] = items
		}), nil, 400},
		{"duplicate observation IDs", mutate(func(d map[string]any) { o := observation(d); d["observations"] = []any{o, o} }), nil, 400},
		{"numeric price", mutate(func(d map[string]any) { observation(d)["offers"].([]any)[0].(map[string]any)["amount"] = 24.90 }), nil, 400},
		{"unknown nested field", mutate(func(d map[string]any) { observation(d)["product"].(map[string]any)["unexpected"] = true }), nil, 400},
		{"missing nullable field", mutate(func(d map[string]any) { delete(observation(d)["product"].(map[string]any), "brand") }), nil, 400},
		{"credential URL", mutate(func(d map[string]any) { observation(d)["source_url"] = "https://user:secret@example.com/path" }), nil, 400},
		{"no hostname", mutate(func(d map[string]any) { observation(d)["product_url"] = "https://:443/path" }), nil, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router, err := NewRouter(storeFunc(func(context.Context, Submission) (bool, error) {
				t.Error("invalid request reached storage")
				return true, nil
			}), testAuth())
			if err != nil {
				t.Fatal(err)
			}
			response := request(router, tc.body, tc.headers)
			if response.Code != tc.status {
				t.Fatalf("status %d; want %d: %s", response.Code, tc.status, response.Body.String())
			}
		})
	}
}

func TestExamplesAndExactByteLimit(t *testing.T) {
	router, err := NewRouter(storeFunc(func(context.Context, Submission) (bool, error) { return true, nil }), testAuth())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"submission", "unavailable"} {
		t.Run(name, func(t *testing.T) {
			body := example(t, name)
			body = append(body, bytes.Repeat([]byte(" "), MaxBodyBytes-len(body))...)
			response := request(router, body, map[string]string{"Content-Type": "application/json; charset=UTF-8", "Content-Encoding": "identity"})
			if response.Code != http.StatusAccepted {
				t.Fatalf("valid example at byte limit rejected: %d %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestNoAcknowledgementBeforeStoreReturns(t *testing.T) {
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	router, err := NewRouter(storeFunc(func(context.Context, Submission) (bool, error) { close(entered); <-release; return true, nil }), testAuth())
	if err != nil {
		t.Fatal(err)
	}
	body := example(t, "submission")
	go func() { defer close(done); request(router, body, nil) }()
	<-entered
	select {
	case <-done:
		t.Error("handler acknowledged before storage completed")
	default:
	}
	close(release)
	<-done
}

func TestAuthConfigurationRequired(t *testing.T) {
	for _, auth := range []Auth{{}, {Token: "secret", ScraperID: "scraper"}, {Token: "secret", ShopIDs: []string{"shop"}}, {Token: "bad token", ScraperID: "scraper", ShopIDs: []string{"shop"}}} {
		if _, err := NewRouter(nil, auth); err == nil {
			t.Errorf("invalid auth config accepted: %+v", auth)
		}
	}
	values := map[string]string{"INGESTION_TOKEN": "secret", "INGESTION_SCRAPER_ID": "scraper", "INGESTION_SHOP_IDS": "one, two"}
	auth := AuthFromEnv(func(key string) string { return values[key] })
	if auth.Token != "secret" || auth.ScraperID != "scraper" || strings.Join(auth.ShopIDs, ",") != "one, two" {
		t.Fatal("environment config not loaded")
	}
	if _, err := NewRouter(nil, auth); err != nil {
		t.Fatal(err)
	}
}
