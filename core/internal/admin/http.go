package admin

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type handler struct {
	database *sql.DB
	username string
	password password
}

func RegisterRoutes(router *gin.Engine, database *sql.DB, getenv func(string) string) error {
	username, encoded := getenv("ADMIN_USERNAME"), getenv("ADMIN_PASSWORD_HASH")
	if username == "" && encoded == "" {
		return nil
	}
	hash, err := parsePassword(encoded)
	if err != nil {
		return err
	}
	if username == "" {
		return ErrCredentials
	}
	owner := handler{database, username, hash}
	owner.register(router)
	return nil
}

func (owner handler) register(router *gin.Engine) {
	routes := router.Group("/api/v1/admin", bounded, origin)
	routes.POST("/session", owner.login)
	protected := routes.Group("", owner.authenticate)
	protected.GET("/session", owner.session)
	protected.DELETE("/session", owner.logout)
	protected.GET("/candidates", owner.candidates)
	protected.GET("/products", owner.products)
	protected.GET("/products/:id", owner.product)
	protected.GET("/groups", owner.candidates)
	protected.GET("/groups/:id", owner.group)
	protected.POST("/reviews", owner.review)
	protected.POST("/groups", owner.review)
}

func bounded(request *gin.Context) {
	incoming := request.Request
	requestContext, cancel := context.WithTimeout(incoming.Context(), 30*time.Second)
	defer cancel()
	request.Request = incoming.WithContext(requestContext)
	request.Request.Body = http.MaxBytesReader(request.Writer, incoming.Body, 8192)
	request.Header("Cache-Control", "no-store")
	request.Header("X-Content-Type-Options", "nosniff")
	request.Next()
}

func fail(request *gin.Context, status int, code string) {
	request.AbortWithStatusJSON(status, gin.H{"error": code})
}
