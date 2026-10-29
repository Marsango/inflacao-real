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
	"github.com/santhosh-tekuri/jsonschema/v6"

	"inflacao-real/core/internal/ingestion/document"
)

const MaxBodyBytes = 1 << 20

type Auth struct {
	Token     string
	ScraperID string
	ShopIDs   []string
}

func AuthFromEnv(environment func(string) string) Auth {
	return Auth{Token: environment("INGESTION_TOKEN"), ScraperID: environment("INGESTION_SCRAPER_ID"),
		ShopIDs: strings.Split(environment("INGESTION_SHOP_IDS"), ",")}
}

func (auth Auth) validate() error {
	if strings.TrimSpace(auth.Token) == "" || strings.ContainsAny(auth.Token, " \t\r\n") || strings.TrimSpace(auth.ScraperID) == "" {
		return errors.New("set INGESTION_TOKEN and INGESTION_SCRAPER_ID")
	}
	if len(auth.allowedShops()) == 0 {
		return errors.New("set INGESTION_SHOP_IDS to at least one allowed shop")
	}
	return nil
}

func (auth Auth) allowedShops() map[string]bool {
	shops := make(map[string]bool)
	for _, identity := range auth.ShopIDs {
		registerShop(shops, identity)
	}
	return shops
}

func registerShop(shops map[string]bool, identity string) {
	if identity = strings.TrimSpace(identity); identity != "" {
		shops[identity] = true
	}
}

type submissionHandler struct {
	store     Store
	scraperID string
	shops     map[string]bool
	schema    *jsonschema.Schema
}

func NewRouter(store Store, auth Auth) (*gin.Engine, error) {
	if err := auth.validate(); err != nil {
		return nil, err
	}
	schema, err := document.CompileSchema()
	if err != nil {
		return nil, err
	}
	handler := submissionHandler{store, auth.ScraperID, auth.allowedShops(), schema}
	router := gin.New()
	router.Use(gin.Recovery())
	router.POST("/api/v1/ingestion/submissions", auth.Authenticate, validateMediaType,
		handler.readSubmission, handler.enqueue)
	return router, nil
}

func fail(request *gin.Context, status int, code string) {
	request.AbortWithStatusJSON(status, gin.H{"error": code})
}

func (auth Auth) Authenticate(request *gin.Context) {
	fields := strings.Fields(request.GetHeader("Authorization"))
	if validBearer(fields, sha256.Sum256([]byte(auth.Token))) {
		return
	}
	request.Header("WWW-Authenticate", "Bearer")
	fail(request, http.StatusUnauthorized, "unauthorized")
}

func validBearer(fields []string, expected [32]byte) bool {
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
		return false
	}
	provided := sha256.Sum256([]byte(fields[1]))
	return subtle.ConstantTimeCompare(provided[:], expected[:]) == 1
}

func validateMediaType(request *gin.Context) {
	mediaType, parameters, err := mime.ParseMediaType(request.GetHeader("Content-Type"))
	charset := parameters["charset"]
	if err != nil || mediaType != "application/json" || (charset != "" && !strings.EqualFold(charset, "utf-8")) {
		fail(request, http.StatusUnsupportedMediaType, "unsupported_media_type")
		return
	}
	encoding := request.GetHeader("Content-Encoding")
	if encoding != "" && !strings.EqualFold(strings.TrimSpace(encoding), "identity") {
		fail(request, http.StatusUnsupportedMediaType, "unsupported_content_encoding")
	}
}

func readBody(request *gin.Context) ([]byte, error) {
	incoming := request.Request
	incoming.Body = http.MaxBytesReader(request.Writer, incoming.Body, MaxBodyBytes)
	return io.ReadAll(incoming.Body)
}

func rejectReadError(request *gin.Context, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		fail(request, http.StatusRequestEntityTooLarge, "payload_too_large")
		return
	}
	fail(request, http.StatusBadRequest, "invalid_payload")
}

func (handler submissionHandler) readSubmission(request *gin.Context) {
	body, err := readBody(request)
	if err != nil {
		rejectReadError(request, err)
		return
	}
	submission, err := document.ValidatePayload(handler.schema, body)
	if err != nil {
		fail(request, http.StatusBadRequest, "invalid_payload")
		return
	}
	if submission.ScraperID != handler.scraperID || !handler.shops[submission.ShopID] {
		fail(request, http.StatusForbidden, "forbidden_identity")
		return
	}
	request.Set("submission", submission)
}

func (handler submissionHandler) enqueue(request *gin.Context) {
	submission := request.MustGet("submission").(Submission)
	incoming := request.Request
	requestContext, cancel := context.WithTimeout(incoming.Context(), 10*time.Second)
	defer cancel()
	inserted, err := handler.store.Enqueue(requestContext, submission)
	if err != nil {
		logEnqueueFailure(requestContext, submission, err)
		fail(request, http.StatusInternalServerError, "ingestion_unavailable")
		return
	}
	logEnqueueSuccess(requestContext, submission, inserted)
	request.Status(submissionStatus(inserted))
}

func submissionStatus(inserted bool) int {
	if inserted {
		return http.StatusAccepted
	}
	return http.StatusOK
}

func logEnqueueFailure(requestContext context.Context, submission Submission, err error) {
	slog.ErrorContext(requestContext, "ingestion persistence failed", "scraper_id", submission.ScraperID,
		"submission_id", submission.SubmissionID, "error", err)
}

func logEnqueueSuccess(requestContext context.Context, submission Submission, inserted bool) {
	slog.InfoContext(requestContext, "ingestion submission received", "scraper_id", submission.ScraperID,
		"submission_id", submission.SubmissionID, "shop_id", submission.ShopID, "inserted", inserted)
}
