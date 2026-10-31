package admin

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

var ErrCredentials = errors.New("ADMIN_USERNAME and ADMIN_PASSWORD_HASH must both be configured")

const cookieName = "__Host-nossa-admin"

func origin(request *gin.Context) {
	if request.Request.Method == http.MethodGet {
		request.Next()
		return
	}
	incoming := request.Request
	source, err := url.Parse(incoming.Header.Get("Origin"))
	if err != nil || source.Host != incoming.Host || source.Scheme != "https" || source.User != nil {
		fail(request, http.StatusForbidden, "invalid_origin")
		return
	}
	request.Next()
}

func randomToken() string { return hex.EncodeToString(randomBytes()) }
func randomBytes() []byte {
	value := make([]byte, 32)
	_, err := rand.Read(value)
	if err != nil {
		panic(err)
	}
	return value
}
func tokenHash(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func setSessionCookie(request *gin.Context, value string, age int) {
	http.SetCookie(request.Writer, &http.Cookie{Name: cookieName, Value: value, Path: "/", MaxAge: age,
		Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
}

func (owner handler) authenticate(request *gin.Context) {
	token, err := request.Cookie(cookieName)
	if err != nil || len(token) != 64 {
		fail(request, 401, "login_required")
		return
	}
	var csrf string
	err = owner.database.QueryRowContext(request.Request.Context(), `SELECT csrf_token FROM admin_sessions WHERE token_hash=$1 AND expires_at>now()`, tokenHash(token)).Scan(&csrf)
	if err != nil {
		fail(request, 401, "login_required")
		return
	}
	if request.Request.Method != http.MethodGet && !equalToken(csrf, request.GetHeader("X-CSRF-Token")) {
		fail(request, 403, "invalid_csrf")
		return
	}
	request.Set("csrf", csrf)
	request.Set("session", tokenHash(token))
	request.Next()
}

func equalToken(first, second string) bool {
	return subtle.ConstantTimeCompare([]byte(first), []byte(second)) == 1
}

func (owner handler) session(request *gin.Context) {
	request.JSON(200, gin.H{"username": owner.username, "csrf": request.GetString("csrf")})
}

func (owner handler) logout(request *gin.Context) {
	_, err := owner.database.ExecContext(request.Request.Context(), `DELETE FROM admin_sessions WHERE token_hash=$1`, request.GetString("session"))
	if err != nil {
		fail(request, 500, "unavailable")
		return
	}
	setSessionCookie(request, "", -1)
	request.Status(http.StatusNoContent)
}

func safeUsername(value string) string { return strings.TrimSpace(value) }
