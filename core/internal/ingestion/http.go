package ingestion

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const MaxBodyBytes = 1 << 20

// Auth configures one scraper installation and its permitted shops. It is loaded
// from environment settings independently of submitted JSON.
type Auth struct {
	Token     string
	ScraperID string
	ShopIDs   []string
}

func AuthFromEnv(getenv func(string) string) Auth {
	return Auth{
		Token:     getenv("INGESTION_TOKEN"),
		ScraperID: getenv("INGESTION_SCRAPER_ID"),
		ShopIDs:   strings.Split(getenv("INGESTION_SHOP_IDS"), ","),
	}
}

func NewRouter(store Store, auth Auth) (*gin.Engine, error) {
	if strings.TrimSpace(auth.Token) == "" || strings.ContainsAny(auth.Token, " \t\r\n") || strings.TrimSpace(auth.ScraperID) == "" {
		return nil, errors.New("set INGESTION_TOKEN and INGESTION_SCRAPER_ID")
	}
	shops := make(map[string]bool)
	for _, id := range auth.ShopIDs {
		if id = strings.TrimSpace(id); id != "" {
			shops[id] = true
		}
	}
	if len(shops) == 0 {
		return nil, errors.New("set INGESTION_SHOP_IDS to at least one allowed shop")
	}
	schema, err := compileSchema()
	if err != nil {
		return nil, err
	}
	tokenHash := sha256.Sum256([]byte(auth.Token))
	router := gin.New()
	router.Use(gin.Recovery())
	router.POST("/api/v1/ingestion/submissions", func(c *gin.Context) {
		fail := func(status int, code string) {
			c.AbortWithStatusJSON(status, gin.H{"error": code})
		}
		fields := strings.Fields(c.GetHeader("Authorization"))
		if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
			c.Header("WWW-Authenticate", "Bearer")
			fail(http.StatusUnauthorized, "unauthorized")
			return
		}
		providedHash := sha256.Sum256([]byte(fields[1]))
		if subtle.ConstantTimeCompare(providedHash[:], tokenHash[:]) != 1 {
			c.Header("WWW-Authenticate", "Bearer")
			fail(http.StatusUnauthorized, "unauthorized")
			return
		}
		mediaType, params, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
		if err != nil || mediaType != "application/json" || (params["charset"] != "" && !strings.EqualFold(params["charset"], "utf-8")) {
			fail(http.StatusUnsupportedMediaType, "unsupported_media_type")
			return
		}
		if encoding := c.GetHeader("Content-Encoding"); encoding != "" && !strings.EqualFold(strings.TrimSpace(encoding), "identity") {
			fail(http.StatusUnsupportedMediaType, "unsupported_content_encoding")
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxBodyBytes)
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				fail(http.StatusRequestEntityTooLarge, "payload_too_large")
			} else {
				fail(http.StatusBadRequest, "invalid_payload")
			}
			return
		}
		submission, err := validatePayload(schema, body)
		if err != nil {
			fail(http.StatusBadRequest, "invalid_payload")
			return
		}
		if submission.ScraperID != auth.ScraperID || !shops[submission.ShopID] {
			fail(http.StatusForbidden, "forbidden_identity")
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()
		inserted, err := store.Enqueue(ctx, submission)
		if err != nil {
			slog.ErrorContext(ctx, "ingestion persistence failed", "scraper_id", submission.ScraperID,
				"submission_id", submission.SubmissionID, "error", err)
			fail(http.StatusInternalServerError, "ingestion_unavailable")
			return
		}
		status := http.StatusOK
		if inserted {
			status = http.StatusAccepted
		}
		slog.InfoContext(ctx, "ingestion submission received", "scraper_id", submission.ScraperID,
			"submission_id", submission.SubmissionID, "shop_id", submission.ShopID, "inserted", inserted)
		c.Status(status)
	})
	return router, nil
}
