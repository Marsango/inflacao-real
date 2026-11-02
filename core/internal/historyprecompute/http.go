package historyprecompute

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type refreshHandler struct {
	store     *Store
	scraperID string
	shops     []string
}

type refreshRequest struct {
	ScraperID string `json:"scraper_id"`
	ShopID    string `json:"shop_id"`
}

func RegisterRoutes(router *gin.Engine, store *Store, authenticate gin.HandlerFunc, scraperID string, shops []string) {
	allowed := make([]string, len(shops))
	for position, shop := range shops {
		allowed[position] = strings.TrimSpace(shop)
	}
	handler := refreshHandler{store, scraperID, allowed}
	router.POST("/api/v1/ingestion/history-refresh", authenticate, handler.request)
}

func (handler refreshHandler) request(request *gin.Context) {
	var parameters refreshRequest
	request.Request.Body = http.MaxBytesReader(request.Writer, request.Request.Body, 4096)
	if err := decodeRefresh(request, &parameters); err != nil {
		request.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid_refresh_request"})
		return
	}
	if parameters.ScraperID != handler.scraperID || !slices.Contains(handler.shops, parameters.ShopID) {
		request.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden_identity"})
		return
	}
	handler.enqueue(request, parameters)
}

func (handler refreshHandler) enqueue(request *gin.Context, parameters refreshRequest) {
	requestContext, cancel := context.WithTimeout(request.Request.Context(), 5*time.Second)
	defer cancel()
	_, err := handler.store.database.ExecContext(requestContext, `UPDATE history_refresh_requests
  SET generation=generation+1,requested_at=now()`)
	if err != nil {
		slog.ErrorContext(requestContext, "request history refresh", "error", err)
		request.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "refresh_unavailable"})
		return
	}
	slog.InfoContext(requestContext, "history refresh requested", "scraper_id", parameters.ScraperID, "shop_id", parameters.ShopID)
	handler.store.notify()
	request.Status(http.StatusAccepted)
}

func decodeRefresh(request *gin.Context, parameters *refreshRequest) error {
	decoder := json.NewDecoder(request.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(parameters); err != nil {
		return err
	}
	if parameters.ScraperID == "" || parameters.ShopID == "" {
		return errors.New("missing refresh identity")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return io.ErrUnexpectedEOF
	}
	return nil
}
