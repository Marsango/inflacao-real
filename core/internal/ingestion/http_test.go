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

func (function storeFunc) Enqueue(requestContext context.Context, submission Submission) (bool, error) {
	return function(requestContext, submission)
}

func requireNoError(test *testing.T, err error) {
	test.Helper()
	if err != nil {
		test.Fatal(err)
	}
}

func example(test *testing.T, name string) []byte {
	test.Helper()
	body, err := os.ReadFile("../../../docs/contracts/ingestion/v1/examples/" + name + ".json")
	requireNoError(test, err)
	return body
}

func testAuth() Auth {
	return Auth{Token: "test-secret", ScraperID: "collector-sp-01", ShopIDs: []string{"mercado-exemplo"}}
}

func request(handler http.Handler, body []byte, headers map[string]string) *httptest.ResponseRecorder {
	incoming := httptest.NewRequest(http.MethodPost, "/api/v1/ingestion/submissions", bytes.NewReader(body))
	incoming.Header.Set("Authorization", "Bearer test-secret")
	incoming.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		incoming.Header.Set(key, value)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, incoming)
	return response
}

type responseCase struct {
	name     string
	inserted bool
	failure  error
	status   int
}

func responseCases() []responseCase {
	return []responseCase{{"new", true, nil, http.StatusAccepted}, {"duplicate", false, nil, http.StatusOK},
		{"database failure", false, errors.New("private database detail"), http.StatusInternalServerError}}
}

func TestSubmissionResponses(test *testing.T) {
	for _, scenario := range responseCases() {
		test.Run(scenario.name, func(test *testing.T) { checkSubmissionResponse(test, scenario) })
	}
}

type submissionRecorder struct {
	test     *testing.T
	body     []byte
	scenario responseCase
	calls    int
}

func checkSubmissionResponse(test *testing.T, scenario responseCase) {
	recorder := &submissionRecorder{test: test, body: example(test, "submission"), scenario: scenario}
	router, err := NewRouter(storeFunc(recorder.enqueue), testAuth())
	requireNoError(test, err)
	response := request(router, recorder.body, nil)
	recorder.assertResponse(response)
}

func (recorder *submissionRecorder) enqueue(requestContext context.Context, submission Submission) (bool, error) {
	recorder.calls++
	recorder.assertMetadata(submission)
	if _, exists := requestContext.Deadline(); !exists {
		recorder.test.Error("database context must have a deadline")
	}
	return recorder.scenario.inserted, recorder.scenario.failure
}

func (recorder *submissionRecorder) assertMetadata(submission Submission) {
	if submission.ScraperID != "collector-sp-01" || submission.ShopID != "mercado-exemplo" || submission.SchemaVersion != "1.0" || submission.SubmissionID == "" || submission.CreatedAt == "" || !bytes.Equal(submission.Payload, recorder.body) {
		recorder.test.Errorf("metadata or original payload was lost: %+v", submission)
	}
}

func (recorder *submissionRecorder) assertResponse(response *httptest.ResponseRecorder) {
	if response.Code != recorder.scenario.status || recorder.calls != 1 {
		recorder.test.Fatalf("status %d, store calls %d; want %d and 1", response.Code, recorder.calls, recorder.scenario.status)
	}
	expected := ""
	if recorder.scenario.failure != nil {
		expected = `{"error":"ingestion_unavailable"}`
	}
	if response.Body.String() != expected {
		recorder.test.Fatal("unexpected response body or leaked persistence error")
	}
}

type invalidRequestCase struct {
	name    string
	body    []byte
	headers map[string]string
	status  int
}

func TestRejectsInvalidRequestsBeforeStorage(test *testing.T) {
	for _, scenario := range invalidRequestCases(test) {
		test.Run(scenario.name, func(test *testing.T) { checkInvalidRequest(test, scenario) })
	}
}

func checkInvalidRequest(test *testing.T, scenario invalidRequestCase) {
	store := storeFunc(func(context.Context, Submission) (bool, error) {
		test.Error("invalid request reached storage")
		return true, nil
	})
	router, err := NewRouter(store, testAuth())
	requireNoError(test, err)
	response := request(router, scenario.body, scenario.headers)
	if response.Code != scenario.status {
		test.Fatalf("status %d; want %d: %s", response.Code, scenario.status, response.Body.String())
	}
}

func invalidRequestCases(test *testing.T) []invalidRequestCase {
	body := example(test, "submission")
	cases := invalidHeaderCases(body)
	cases = append(cases, invalidJSONCases(body)...)
	cases = append(cases, invalidEnvelopeCases(test, body)...)
	cases = append(cases, invalidObservationCases(test, body)...)
	return append(cases, invalidIdentityCases(test, body)...)
}

func invalidHeaderCases(body []byte) []invalidRequestCase {
	return []invalidRequestCase{
		{"missing token", body, map[string]string{"Authorization": ""}, 401},
		{"wrong token", body, map[string]string{"Authorization": "Bearer wrong"}, 401},
		{"content type", body, map[string]string{"Content-Type": "text/plain"}, 415},
		{"charset", body, map[string]string{"Content-Type": "application/json; charset=latin1"}, 415},
		{"compressed", body, map[string]string{"Content-Encoding": "gzip"}, 415},
		{"too large", append(bytes.Repeat([]byte(" "), MaxBodyBytes), body...), nil, 413},
	}
}

func invalidJSONCases(body []byte) []invalidRequestCase {
	marker := []byte(`"schema_version": "1.0"`)
	return []invalidRequestCase{
		{"empty", nil, nil, 400}, {"malformed", []byte(`{"schema_version":`), nil, 400},
		{"trailing document", append(bytes.Clone(body), []byte("{}")...), nil, 400},
		{"duplicate key", bytes.Replace(body, marker, []byte(`"schema_version":"1.0","schema_version":"1.0"`), 1), nil, 400},
		{"escaped duplicate key", bytes.Replace(body, marker, []byte(`"schema_version":"1.0","schema_versio\u006e":"1.0"`), 1), nil, 400},
		{"unpaired surrogate", bytes.Replace(body, marker, []byte(`"schema_version":"\ud800"`), 1), nil, 400},
		{"invalid UTF8", append(bytes.Clone(body), 0xff), nil, 400},
	}
}

func mutateDocument(test *testing.T, body []byte, change func(map[string]any)) []byte {
	test.Helper()
	var document map[string]any
	requireNoError(test, json.Unmarshal(body, &document))
	change(document)
	result, err := json.Marshal(document)
	requireNoError(test, err)
	return result
}

func invalidEnvelopeCases(test *testing.T, body []byte) []invalidRequestCase {
	mutate := func(change func(map[string]any)) []byte { return mutateDocument(test, body, change) }
	return []invalidRequestCase{
		{"unknown field", mutate(func(document map[string]any) { document["status"] = "processed" }), nil, 400},
		{"unknown version", mutate(func(document map[string]any) { document["schema_version"] = "2.0" }), nil, 400},
		{"missing required", mutate(func(document map[string]any) { delete(document, "run_id") }), nil, 400},
		{"invalid calendar date", mutate(func(document map[string]any) { document["created_at"] = "2026-02-30T12:00:00Z" }), nil, 400},
		{"non UTC timestamp", mutate(func(document map[string]any) { document["created_at"] = "2026-10-27T09:00:00-03:00" }), nil, 400},
		{"empty observations", mutate(func(document map[string]any) { document["observations"] = []any{} }), nil, 400},
		{"too many observations", mutate(excessObservations), nil, 400},
		{"duplicate observation IDs", mutate(duplicateObservations), nil, 400},
	}
}

func firstObservation(document map[string]any) map[string]any {
	return document["observations"].([]any)[0].(map[string]any)
}

func excessObservations(document map[string]any) {
	items := make([]any, 101)
	for position := range items {
		items[position] = firstObservation(document)
	}
	document["observations"] = items
}

func duplicateObservations(document map[string]any) {
	observation := firstObservation(document)
	document["observations"] = []any{observation, observation}
}

func invalidObservationCases(test *testing.T, body []byte) []invalidRequestCase {
	mutate := func(change func(map[string]any)) []byte { return mutateDocument(test, body, change) }
	return []invalidRequestCase{
		{"numeric price", mutate(numericPrice), nil, 400},
		{"unknown nested field", mutate(func(document map[string]any) {
			firstObservation(document)["product"].(map[string]any)["unexpected"] = true
		}), nil, 400},
		{"missing nullable field", mutate(func(document map[string]any) { delete(firstObservation(document)["product"].(map[string]any), "brand") }), nil, 400},
		{"credential URL", mutate(func(document map[string]any) {
			firstObservation(document)["source_url"] = "https://user:secret@example.com/path"
		}), nil, 400},
		{"no hostname", mutate(func(document map[string]any) { firstObservation(document)["product_url"] = "https://:443/path" }), nil, 400},
	}
}

func numericPrice(document map[string]any) {
	firstObservation(document)["offers"].([]any)[0].(map[string]any)["amount"] = 24.90
}

func invalidIdentityCases(test *testing.T, body []byte) []invalidRequestCase {
	mutate := func(change func(map[string]any)) []byte { return mutateDocument(test, body, change) }
	return []invalidRequestCase{
		{"different scraper", mutate(func(document map[string]any) { document["scraper_id"] = "other-scraper" }), nil, 403},
		{"different shop", mutate(func(document map[string]any) { document["shop_id"] = "other-shop" }), nil, 403},
	}
}

func TestExamplesAndExactByteLimit(test *testing.T) {
	router, err := NewRouter(storeFunc(func(context.Context, Submission) (bool, error) { return true, nil }), testAuth())
	requireNoError(test, err)
	for _, name := range []string{"submission", "unavailable"} {
		test.Run(name, func(test *testing.T) { checkByteLimit(test, router, name) })
	}
}

func checkByteLimit(test *testing.T, handler http.Handler, name string) {
	body := example(test, name)
	body = append(body, bytes.Repeat([]byte(" "), MaxBodyBytes-len(body))...)
	headers := map[string]string{"Content-Type": "application/json; charset=UTF-8", "Content-Encoding": "identity"}
	response := request(handler, body, headers)
	if response.Code != http.StatusAccepted {
		test.Fatalf("valid example at byte limit rejected: %d %s", response.Code, response.Body.String())
	}
}

func TestNoAcknowledgementBeforeStoreReturns(test *testing.T) {
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	store := storeFunc(func(context.Context, Submission) (bool, error) { close(entered); <-release; return true, nil })
	router, err := NewRouter(store, testAuth())
	requireNoError(test, err)
	body := example(test, "submission")
	go func() { defer close(done); request(router, body, nil) }()
	<-entered
	select {
	case <-done:
		test.Error("handler acknowledged before storage completed")
	default:
	}
	close(release)
	<-done
}

func invalidAuthCases() []Auth {
	return []Auth{{}, {Token: "secret", ScraperID: "scraper"}, {Token: "secret", ShopIDs: []string{"shop"}},
		{Token: "bad token", ScraperID: "scraper", ShopIDs: []string{"shop"}}}
}

func TestAuthConfigurationRequired(test *testing.T) {
	for _, auth := range invalidAuthCases() {
		assertInvalidAuth(test, auth)
	}
	values := map[string]string{"INGESTION_TOKEN": "secret", "INGESTION_SCRAPER_ID": "scraper", "INGESTION_SHOP_IDS": "one, two"}
	auth := AuthFromEnv(func(key string) string { return values[key] })
	if auth.Token != "secret" || auth.ScraperID != "scraper" || strings.Join(auth.ShopIDs, ",") != "one, two" {
		test.Fatal("environment config not loaded")
	}
	_, err := NewRouter(nil, auth)
	requireNoError(test, err)
}

func assertInvalidAuth(test *testing.T, auth Auth) {
	if _, err := NewRouter(nil, auth); err == nil {
		test.Errorf("invalid auth config accepted: %+v", auth)
	}
}
