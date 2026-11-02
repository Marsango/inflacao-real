package ingestion

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"inflacao-real/core/internal/dailysummary"
	"inflacao-real/core/internal/ingestion/document"
)

type queuedPayload struct {
	identity                                       int64
	attempts                                       int
	raw                                            []byte
	scraperID, submissionID, shopID, schemaVersion string
}

func readQueuedPayload(requestContext context.Context, transaction *sql.Tx) (queuedPayload, error) {
	var payload queuedPayload
	row := transaction.QueryRowContext(requestContext, nextPayloadSQL)
	err := row.Scan(&payload.identity, &payload.attempts, &payload.raw, &payload.scraperID,
		&payload.submissionID, &payload.shopID, &payload.schemaVersion)
	return payload, err
}

func (worker *Worker) processTransaction(parentContext, processingContext context.Context, transaction *sql.Tx) (bool, error) {
	payload, err := readQueuedPayload(processingContext, transaction)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("select pending payload: %w", err)
	}
	return true, worker.processSelected(parentContext, processingContext, transaction, payload)
}

func (worker *Worker) processSelected(parentContext, processingContext context.Context, transaction *sql.Tx, payload queuedPayload) error {
	err := payload.commit(processingContext, transaction, worker.schema)
	if err == nil {
		payload.logSuccess(parentContext)
		return nil
	}
	transaction.Rollback()
	if parentContext.Err() != nil {
		return parentContext.Err()
	}
	return worker.recordFailure(parentContext, payload, err)
}

func (payload queuedPayload) decode(schema *jsonschema.Schema) (document.Envelope, error) {
	envelope, err := document.DecodeEnvelope(schema, payload.raw)
	if err != nil {
		return envelope, err
	}
	if envelope.ScraperID != payload.scraperID || envelope.SubmissionID != payload.submissionID || envelope.ShopID != payload.shopID || payload.schemaVersion != "1.0" {
		return envelope, document.Permanent("stored payload identity does not match queue metadata")
	}
	return envelope, nil
}

func (payload queuedPayload) commit(requestContext context.Context, transaction *sql.Tx, schema *jsonschema.Schema) error {
	envelope, err := payload.decode(schema)
	if err != nil {
		return err
	}
	if err := persistEnvelope(requestContext, transaction, payload.identity, envelope); err != nil {
		return err
	}
	if _, err := transaction.ExecContext(requestContext, processedPayloadSQL, payload.identity); err != nil {
		return err
	}
	if err := dailysummary.MarkPayload(requestContext, transaction, payload.identity); err != nil {
		return err
	}
	return transaction.Commit()
}

func (payload queuedPayload) logSuccess(requestContext context.Context) {
	slog.InfoContext(requestContext, "ingestion payload processed", "payload_id", payload.identity,
		"scraper_id", payload.scraperID, "submission_id", payload.submissionID, "attempt", payload.attempts+1)
}

func (worker *Worker) recordFailure(requestContext context.Context, payload queuedPayload, cause error) error {
	failureContext, cancel := context.WithTimeout(requestContext, 10*time.Second)
	defer cancel()
	terminal := document.IsPermanent(cause) || payload.attempts+1 >= worker.config.MaxAttempts
	delay := worker.config.retryDelay(payload.attempts + 1)
	_, err := worker.database.ExecContext(failureContext, failedPayloadSQL, payload.identity,
		payload.attempts, cause.Error(), delay.Microseconds(), terminal)
	if err != nil {
		return fmt.Errorf("persist failure for payload %d (%v): %w", payload.identity, cause, err)
	}
	payload.logFailure(requestContext, terminal, cause)
	return nil
}

func (payload queuedPayload) logFailure(requestContext context.Context, terminal bool, cause error) {
	slog.ErrorContext(requestContext, "ingestion payload processing failed", "payload_id", payload.identity,
		"scraper_id", payload.scraperID, "submission_id", payload.submissionID, "attempt", payload.attempts+1,
		"terminal", terminal, "error", cause)
}
