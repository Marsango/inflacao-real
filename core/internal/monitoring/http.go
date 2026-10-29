package monitoring

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Store interface {
	List(context.Context) ([]Target, error)
}

func RegisterRoutes(router *gin.Engine, store Store, authenticate gin.HandlerFunc) {
	router.GET("/api/v1/monitoring-targets", authenticate, handler{store}.list)
}

type handler struct{ store Store }

func (handler handler) list(request *gin.Context) {
	incoming := request.Request
	requestContext, cancel := context.WithTimeout(incoming.Context(), 10*time.Second)
	defer cancel()
	targets, err := handler.store.List(requestContext)
	if err != nil {
		slog.ErrorContext(requestContext, "monitoring targets unavailable", "error", err)
		request.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "monitoring_unavailable"})
		return
	}
	request.Header("Cache-Control", "no-store")
	request.JSON(http.StatusOK, Scope{SchemaVersion: "1.0", Targets: targets})
}
