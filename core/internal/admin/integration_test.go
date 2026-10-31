package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"inflacao-real/core/internal/database"
	"inflacao-real/core/internal/testdatabase"
)

type sessionFixture struct {
	test   *testing.T
	router http.Handler
	cookie *http.Cookie
	csrf   string
}

func adminFixture(test *testing.T) (sessionFixture, handler) {
	_, pool := testdatabase.New(test, database.Open, database.Migrate)
	hash, err := parsePassword(testHash())
	if err != nil {
		test.Fatal(err)
	}
	owner := handler{pool, "owner", hash}
	router := gin.New()
	owner.register(router)
	return sessionFixture{test: test, router: router}, owner
}
func (fixture sessionFixture) request(method, path string, body any, csrf string) *httptest.ResponseRecorder {
	fixture.test.Helper()
	encoded, _ := json.Marshal(body)
	incoming := httptest.NewRequest(method, "https://example.com/api/v1/admin/"+path, bytes.NewReader(encoded))
	incoming.Header.Set("Origin", "https://example.com")
	incoming.Header.Set("Content-Type", "application/json")
	incoming.Header.Set("X-CSRF-Token", csrf)
	if fixture.cookie != nil {
		incoming.AddCookie(fixture.cookie)
	}
	response := httptest.NewRecorder()
	fixture.router.ServeHTTP(response, incoming)
	return response
}
func (fixture *sessionFixture) login() {
	fixture.test.Helper()
	response := fixture.request("POST", "session", credentials{"owner", "owner-password"}, "")
	if response.Code != 200 {
		fixture.test.Fatal(response.Code, response.Body)
	}
	cookies := response.Result().Cookies()
	fixture.cookie = cookies[0]
	var result map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		fixture.test.Fatal(err)
	}
	fixture.csrf = result["csrf"]
}
func (fixture sessionFixture) status(response *httptest.ResponseRecorder, expected int) {
	fixture.test.Helper()
	if response.Code != expected {
		fixture.test.Fatalf("expected %d: %d %s", expected, response.Code, response.Body)
	}
}

func TestSessionAuthenticationCSRFAndLogout(test *testing.T) {
	fixture, _ := adminFixture(test)
	fixture.status(fixture.request("GET", "candidates", nil, ""), 401)
	fixture.login()
	cookie := fixture.cookie
	if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.MaxAge != 28800 {
		test.Fatal("unsafe cookie", cookie)
	}
	fixture.status(fixture.request("GET", "candidates", nil, ""), 200)
	fixture.status(fixture.request("POST", "reviews", map[string]any{}, ""), 403)
	fixture.status(fixture.request("POST", "reviews", map[string]any{}, "wrong"), 403)
	fixture.status(fixture.request("DELETE", "session", nil, fixture.csrf), 204)
	fixture.status(fixture.request("GET", "session", nil, ""), 401)
}

func TestSessionExpiryAndLoginThrottle(test *testing.T) {
	fixture, owner := adminFixture(test)
	fixture.login()
	if _, err := owner.database.Exec(`UPDATE admin_sessions SET expires_at=now()-interval '1 second'`); err != nil {
		test.Fatal(err)
	}
	fixture.status(fixture.request("GET", "session", nil, ""), 401)
	for range 9 {
		fixture.status(fixture.request("POST", "session", credentials{"owner", "wrong"}, ""), 401)
	}
	fixture.status(fixture.request("POST", "session", credentials{"owner", "owner-password"}, ""), 429)
}

func TestAdminRejectsStaleCatalogReview(test *testing.T) {
	fixture, owner := adminFixture(test)
	fixture.login()
	if _, err := owner.database.Exec(`UPDATE catalog_decisions SET revision=1`); err != nil {
		test.Fatal(err)
	}
	fixture.status(fixture.request("POST", "reviews", map[string]any{"revision": 0, "action": "activate", "reason": "Revisado"}, fixture.csrf), 409)
	fixture.status(fixture.request("POST", "reviews", map[string]any{"revision": 1, "action": "activate", "reason": "Revisado"}, fixture.csrf), 204)
}
