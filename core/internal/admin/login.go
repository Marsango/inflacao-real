package admin

import (
	"context"

	"github.com/gin-gonic/gin"
)

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (owner handler) login(request *gin.Context) {
	var input credentials
	if err := request.ShouldBindJSON(&input); err != nil {
		fail(request, 400, "invalid_request")
		return
	}
	allowed, err := owner.allowLogin(request.Request.Context())
	if err != nil {
		fail(request, 503, "unavailable")
		return
	}
	if !allowed {
		request.Header("Retry-After", "900")
		fail(request, 429, "login_throttled")
		return
	}
	owner.verifyLogin(request, input)
}

func (owner handler) verifyLogin(request *gin.Context, input credentials) {
	valid := owner.password.verify(input.Password)
	if !equalToken(safeUsername(input.Username), owner.username) || !valid {
		fail(request, 401, "invalid_credentials")
		return
	}
	owner.createSession(request)
}

// A shared database counter also bounds Argon2 work across processes and proxy IPs.
func (owner handler) allowLogin(requestContext context.Context) (bool, error) {
	var attempts int
	err := owner.database.QueryRowContext(requestContext, `INSERT INTO admin_login_throttle(identity,attempts,reset_at)
  VALUES($1,1,now()+interval '15 minutes') ON CONFLICT(identity) DO UPDATE SET
  attempts=CASE WHEN admin_login_throttle.reset_at<=now() THEN 1 ELSE admin_login_throttle.attempts+1 END,
  reset_at=CASE WHEN admin_login_throttle.reset_at<=now() THEN now()+interval '15 minutes' ELSE admin_login_throttle.reset_at END
  RETURNING attempts`, owner.username).Scan(&attempts)
	return attempts <= 10, err
}

func (owner handler) createSession(request *gin.Context) {
	token, csrf := randomToken(), randomToken()
	requestContext := request.Request.Context()
	_, err := owner.database.ExecContext(requestContext, `WITH cleanup AS (DELETE FROM admin_sessions WHERE expires_at<=now())
  INSERT INTO admin_sessions(token_hash,csrf_token,expires_at) VALUES($1,$2,now()+interval '8 hours')`, tokenHash(token), csrf)
	if err != nil {
		fail(request, 500, "unavailable")
		return
	}
	setSessionCookie(request, token, 8*60*60)
	request.JSON(200, gin.H{"username": owner.username, "csrf": csrf})
}
