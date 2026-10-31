package admin

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/argon2"
)

func testHash() string {
	salt := []byte("0123456789abcdef")
	digest := argon2.IDKey([]byte("owner-password"), salt, 2, 19456, 1, 32)
	return "$argon2id$v=19$m=19456,t=2,p=1$" + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(digest)
}

func TestPasswordConfigurationAndVerification(test *testing.T) {
	hash, err := parsePassword(testHash())
	if err != nil {
		test.Fatal(err)
	}
	if !hash.verify("owner-password") || hash.verify("wrong") {
		test.Fatal("password verification failed")
	}
	for _, encoded := range []string{"", strings.Replace(testHash(), "m=19456", "m=1", 1), strings.Replace(testHash(), "argon2id", "argon2i", 1)} {
		assertInvalidPasswordHash(test, encoded)
	}
}

func TestAbsentCredentialsDisableAdmin(test *testing.T) {
	router := gin.New()
	if err := RegisterRoutes(router, nil, func(string) string { return "" }); err != nil {
		test.Fatal(err)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("GET", "/api/v1/admin/session", nil))
	if response.Code != 404 {
		test.Fatal("admin enabled without credentials")
	}
}

func TestOriginRestrictions(test *testing.T) {
	router := gin.New()
	router.POST("/review", origin, func(request *gin.Context) { request.Status(204) })
	for _, source := range []string{"", "https://evil.example", "http://example.com", "https://example.com"} {
		checkOrigin(test, router, source)
	}
}
func checkOrigin(test *testing.T, router http.Handler, source string) {
	test.Helper()
	request := httptest.NewRequest("POST", "https://example.com/review", nil)
	request.Header.Set("Origin", source)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	expected := 403
	if source == "https://example.com" {
		expected = 204
	}
	if response.Code != expected {
		test.Fatal(source, response.Code)
	}
}

func assertInvalidPasswordHash(test *testing.T, encoded string) {
	test.Helper()
	if _, err := parsePassword(encoded); err == nil {
		test.Fatal("unsafe hash accepted")
	}
}
