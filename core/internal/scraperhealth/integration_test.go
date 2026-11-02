package scraperhealth

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"inflacao-real/core/internal/database"
	"inflacao-real/core/internal/ingestion"
	"inflacao-real/core/internal/testdatabase"
)

func fixtureReport() Report {
	return Report{Version: "1.0", ScraperID: "collector", ShopID: "mambo", Name: "Mambo",
		RunID: "11111111-1111-4111-8111-111111111111", Started: time.Now().UTC().Truncate(time.Second),
		Finished: time.Now().UTC(), Errors: 1, Diagnostics: []Diagnostic{{"error", "collection", "target=coffee: mismatch"}}}
}
func TestReportLifecycle(test *testing.T) {
	ctx, pool := testdatabase.New(test, database.Open, database.Migrate)
	_, err := pool.ExecContext(ctx, `INSERT INTO shops(id,name,website_url) VALUES('mambo','Mambo','https://example.com')`)
	if err != nil {
		test.Fatal(err)
	}
	router := gin.New()
	auth := ingestion.Auth{Token: "secret", ScraperID: "collector", ShopIDs: []string{"mambo"}}
	Register(router, pool, auth)
	RegisterAdmin(router.Group("/admin"), pool, auth.ScraperID, auth.ShopIDs)
	report := fixtureReport()
	assertRequest(test, router, "POST", "/api/v1/ingestion/scraper-reports", report, "", 401)
	assertRequest(test, router, "POST", "/api/v1/ingestion/scraper-reports", report, "secret", 201)
	assertRequest(test, router, "POST", "/api/v1/ingestion/scraper-reports", report, "secret", 200)
	verifyLifecycle(test, router, report)
}
func verifyLifecycle(test *testing.T, router *gin.Engine, report Report) {
	path := "/admin/scraper-diagnostics?scraper_id=collector&shop_id=mambo"
	assertRequest(test, router, "DELETE", path, nil, "", 204)
	assertRequest(test, router, "POST", "/api/v1/ingestion/scraper-reports", report, "secret", 200)
	response := assertRequest(test, router, "GET", path, nil, "", 200)
	if response.Body.String() != "[]" {
		test.Fatal("deleted issues resurrected", response.Body)
	}
	report.RunID = "22222222-2222-4222-8222-222222222222"
	report.Started = report.Started.Add(-time.Hour)
	report.Errors, report.Diagnostics = 0, []Diagnostic{}
	assertRequest(test, router, "POST", "/api/v1/ingestion/scraper-reports", report, "secret", 201)
	response = assertRequest(test, router, "GET", "/admin/scrapers", nil, "", 200)
	if !bytes.Contains(response.Body.Bytes(), []byte(`"status":"error"`)) {
		test.Fatal("older report replaced latest health", response.Body)
	}
	if !bytes.Contains(response.Body.Bytes(), []byte(`"scraper_name":"Mambo"`)) {
		test.Fatal(response.Body)
	}
}
func assertRequest(test *testing.T, router *gin.Engine, method, path string, body any, token string, expected int) *httptest.ResponseRecorder {
	test.Helper()
	payload, _ := json.Marshal(body)
	incoming := httptest.NewRequest(method, path, bytes.NewReader(payload))
	incoming.Header.Set("Content-Type", "application/json")
	if token != "" {
		incoming.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, incoming)
	if response.Code != expected {
		test.Fatalf("%s %s: %d %s", method, path, response.Code, response.Body)
	}
	return response
}
func TestValidateReport(test *testing.T) {
	report := fixtureReport()
	if err := report.validate(); err != nil {
		test.Fatal(err)
	}
	report.Errors = 0
	if report.validate() == nil {
		test.Fatal("accepted inconsistent counts")
	}
	report = fixtureReport()
	report.Name = ""
	if report.validate() == nil {
		test.Fatal("accepted missing display name")
	}
	report = fixtureReport()
	report.Finished = report.Started.Add(-time.Second)
	if report.validate() == nil {
		test.Fatal("accepted reversed times")
	}
}

func TestInvalidRequests(test *testing.T) {
	router := gin.New()
	auth := ingestion.Auth{Token: "secret", ScraperID: "collector", ShopIDs: []string{"mambo"}}
	Register(router, nil, auth)
	report := fixtureReport()
	report.ShopID = "festval"
	assertRequest(test, router, "POST", "/api/v1/ingestion/scraper-reports", report, "secret", 403)
	report = fixtureReport()
	report.Name = ""
	assertRequest(test, router, "POST", "/api/v1/ingestion/scraper-reports", report, "secret", 400)
	report = fixtureReport()
	report.Diagnostics[0].Message = strings.Repeat("x", 256*1024)
	assertRequest(test, router, "POST", "/api/v1/ingestion/scraper-reports", report, "secret", 400)
}

func TestEmbeddedContract(test *testing.T) {
	expected, err := os.ReadFile("../../../docs/contracts/scraper-reports/v1/schema.json")
	if err != nil {
		test.Fatal(err)
	}
	if !bytes.Equal(expected, schemaJSON) {
		test.Fatal("embedded diagnostic schema differs from published contract")
	}
}
