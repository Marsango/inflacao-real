package webapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type handler struct{ store Store }

func RegisterRoutes(router *gin.Engine, store Store) {
	handler := handler{store}
	router.GET("/api/v1/products", handler.products)
	router.GET("/api/v1/history", handler.history)
}

func (handler handler) products(request *gin.Context) {
	incoming := request.Request
	incomingURL := incoming.URL
	after, limit, err := pagination(incomingURL.Query())
	if err != nil {
		request.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid_pagination"})
		return
	}
	requestContext, cancel := context.WithTimeout(incoming.Context(), 10*time.Second)
	defer cancel()
	products, err := handler.store.Products(requestContext, after, limit)
	respond(request, products, err)
}

func (handler handler) history(request *gin.Context) {
	incoming := request.Request
	incomingURL := incoming.URL
	parameters, err := historyRequest(incomingURL.Query())
	if err != nil {
		request.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid_history_request"})
		return
	}
	requestContext, cancel := context.WithTimeout(incoming.Context(), 10*time.Second)
	defer cancel()
	history, err := handler.store.History(requestContext, parameters)
	respond(request, history, err)
}

func respond(request *gin.Context, result any, err error) {
	request.Header("Cache-Control", "no-store")
	if errors.Is(err, ErrNotFound) {
		request.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "product_not_found"})
		return
	}
	if err != nil {
		unavailable(request, err)
		return
	}
	request.JSON(http.StatusOK, result)
}

func unavailable(request *gin.Context, err error) {
	incoming := request.Request
	incomingURL := incoming.URL
	slog.ErrorContext(incoming.Context(), "web API unavailable", "path", incomingURL.Path, "error", err)
	request.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "data_unavailable"})
}
