package scraperhealth

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"inflacao-real/core/internal/ingestion"
)

func Register(router *gin.Engine, database *sql.DB, auth ingestion.Auth) {
	router.POST("/api/v1/ingestion/scraper-reports", auth.Authenticate, bounded, func(request *gin.Context) { receive(request, database, auth) })
}
func receive(request *gin.Context, database *sql.DB, auth ingestion.Auth) {
	report, err := decode(request)
	if err != nil {
		request.AbortWithStatusJSON(400, gin.H{"error": "invalid_report"})
		return
	}
	if !authorized(report, auth) {
		request.AbortWithStatus(403)
		return
	}
	inserted, err := Save(request.Request.Context(), database, report)
	if err != nil {
		slog.ErrorContext(request.Request.Context(), "scraper report persistence failed", "scraper_id", report.ScraperID, "shop_id", report.ShopID, "run_id", report.RunID, "error", err)
		request.AbortWithStatusJSON(503, gin.H{"error": "report_unavailable"})
		return
	}
	request.Status(acknowledgement(inserted))
}
func decode(request *gin.Context) (Report, error) {
	var report Report
	media, _, err := mime.ParseMediaType(request.GetHeader("Content-Type"))
	if err != nil || media != "application/json" {
		return report, io.ErrUnexpectedEOF
	}
	request.Request.Body = http.MaxBytesReader(request.Writer, request.Request.Body, 256*1024)
	body, err := io.ReadAll(request.Request.Body)
	if err != nil {
		return report, err
	}
	if err = validateSchema(body); err != nil {
		return report, err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err = decoder.Decode(&report); err != nil {
		return report, err
	}
	return report, report.validate()
}
func authorized(report Report, auth ingestion.Auth) bool {
	if report.ScraperID != auth.ScraperID {
		return false
	}
	return slices.ContainsFunc(auth.ShopIDs, func(shop string) bool { return strings.TrimSpace(shop) == report.ShopID })
}

func acknowledgement(inserted bool) int {
	if inserted {
		return 201
	}
	return 200
}
func bounded(request *gin.Context) {
	ctx, cancel := context.WithTimeout(request.Request.Context(), 10*time.Second)
	defer cancel()
	request.Request = request.Request.WithContext(ctx)
	request.Next()
}
