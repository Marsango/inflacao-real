package ingestion

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

type WorkerConfig struct {
	PollInterval time.Duration
	BatchSize    int
	MaxAttempts  int
	RetryBase    time.Duration
	RetryMax     time.Duration
}

func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{PollInterval: time.Minute, BatchSize: 100, MaxAttempts: 5,
		RetryBase: time.Minute, RetryMax: time.Hour}
}

func WorkerConfigFromEnv(getenv func(string) string) (WorkerConfig, error) {
	c := DefaultWorkerConfig()
	for name, target := range map[string]*time.Duration{
		"INGESTION_POLL_INTERVAL": &c.PollInterval,
		"INGESTION_RETRY_BASE":    &c.RetryBase, "INGESTION_RETRY_MAX": &c.RetryMax,
	} {
		if value := getenv(name); value != "" {
			duration, err := time.ParseDuration(value)
			if err != nil || duration <= 0 {
				return c, fmt.Errorf("%s must be a positive duration", name)
			}
			*target = duration
		}
	}
	for name, target := range map[string]*int{
		"INGESTION_BATCH_SIZE": &c.BatchSize, "INGESTION_MAX_ATTEMPTS": &c.MaxAttempts,
	} {
		if value := getenv(name); value != "" {
			n, err := strconv.Atoi(value)
			if err != nil || n <= 0 {
				return c, fmt.Errorf("%s must be a positive integer", name)
			}
			*target = n
		}
	}
	return c, c.validate()
}

func (c WorkerConfig) validate() error {
	if c.PollInterval <= 0 || c.BatchSize <= 0 || c.MaxAttempts <= 0 || c.RetryBase <= 0 || c.RetryMax < c.RetryBase {
		return errors.New("worker settings must be positive and retry max must be at least retry base")
	}
	return nil
}

func (c WorkerConfig) retryDelay(attempt int) time.Duration {
	delay := c.RetryBase
	for i := 1; i < attempt; i++ {
		if delay >= c.RetryMax-delay {
			return c.RetryMax
		}
		delay *= 2
	}
	return delay
}

type Worker struct {
	db     *sql.DB
	config WorkerConfig
	schema *jsonschema.Schema
}

func NewWorker(db *sql.DB, config WorkerConfig) (*Worker, error) {
	if db == nil {
		return nil, errors.New("worker requires a database")
	}
	if err := config.validate(); err != nil {
		return nil, err
	}
	schema, err := compileSchema()
	if err != nil {
		return nil, fmt.Errorf("compile worker schema: %w", err)
	}
	return &Worker{db: db, config: config, schema: schema}, nil
}

// Run processes immediately, drains ready work sequentially, then waits. The
// caller must cancel and join Run before closing the database pool.
func (w *Worker) Run(ctx context.Context) {
	runLoop(ctx, w.config.BatchSize, w.ProcessNext, func(ctx context.Context) {
		statsCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		var backlog, failed, retries int64
		var oldest float64
		err := w.db.QueryRowContext(statsCtx, `SELECT
			count(*) FILTER (WHERE status='pending'),
			COALESCE(EXTRACT(EPOCH FROM now()-min(received_at) FILTER (WHERE status='pending')),0)::float8,
			count(*) FILTER (WHERE status='failed'),
			COALESCE(sum(GREATEST(attempts-1,0)),0)::bigint FROM ingestion_payloads`).Scan(&backlog, &oldest, &failed, &retries)
		cancel()
		if err == nil {
			slog.InfoContext(ctx, "ingestion queue", "pending", backlog, "oldest_pending_seconds", oldest,
				"failed", failed, "retries", retries)
		} else if ctx.Err() == nil {
			slog.ErrorContext(ctx, "read ingestion queue metrics", "error", err)
		}
		timer := time.NewTimer(w.config.PollInterval)
		defer timer.Stop()
		select {
		case <-ctx.Done():
		case <-timer.C:
		}
	})
}

func runLoop(ctx context.Context, batchSize int, process func(context.Context) (bool, error), idle func(context.Context)) {
	for ctx.Err() == nil {
		processed := 0
		for ; processed < batchSize && ctx.Err() == nil; processed++ {
			found, err := process(ctx)
			if err != nil {
				if ctx.Err() == nil {
					slog.ErrorContext(ctx, "ingestion worker cycle failed", "error", err)
				}
				break
			}
			if !found {
				break
			}
		}
		if ctx.Err() == nil && processed < batchSize {
			idle(ctx)
		}
	}
}

// ProcessNext locks at most one ready payload. A handled input failure returns
// (true, nil), allowing the loop to continue with unrelated submissions.
func (w *Worker) ProcessNext(ctx context.Context) (bool, error) {
	processingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := w.db.BeginTx(processingCtx, nil)
	if err != nil {
		return false, fmt.Errorf("begin payload processing: %w", err)
	}
	defer tx.Rollback()
	var id int64
	var attempts int
	var raw []byte
	var scraperID, submissionID, shopID, schemaVersion string
	err = tx.QueryRowContext(processingCtx, `SELECT id,attempts,payload,scraper_id,submission_id,shop_id,schema_version
		FROM ingestion_payloads WHERE status='pending' AND next_attempt_at<=now()
		ORDER BY received_at,id LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(
		&id, &attempts, &raw, &scraperID, &submissionID, &shopID, &schemaVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("select pending payload: %w", err)
	}
	e, err := decodeEnvelope(w.schema, raw)
	if err == nil && (e.ScraperID != scraperID || e.SubmissionID != submissionID || e.ShopID != shopID || schemaVersion != "1.0") {
		err = permanent("stored payload identity does not match queue metadata")
	}
	if err == nil {
		err = persistEnvelope(processingCtx, tx, id, e)
	}
	if err == nil {
		_, err = tx.ExecContext(processingCtx, `UPDATE ingestion_payloads SET status='processed',
			processed_at=clock_timestamp(),attempts=attempts+1,last_error=NULL WHERE id=$1`, id)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err == nil {
		slog.InfoContext(ctx, "ingestion payload processed", "payload_id", id, "scraper_id", scraperID,
			"submission_id", submissionID, "attempt", attempts+1)
		return true, nil
	}
	tx.Rollback()
	if ctx.Err() != nil {
		// Shutdown rolls back; the pending payload is immediately recoverable.
		return true, ctx.Err()
	}
	failureCtx, failureCancel := context.WithTimeout(ctx, 10*time.Second)
	defer failureCancel()
	terminal := isPermanent(err) || attempts+1 >= w.config.MaxAttempts
	// Compare-and-set protects against a competing worker succeeding or recording
	// a newer failure after rollback. It also handles an ambiguous commit safely.
	_, updateErr := w.db.ExecContext(failureCtx, `UPDATE ingestion_payloads SET
		attempts=attempts+1, last_error=$3,
		next_attempt_at=clock_timestamp()+$4::bigint*interval '1 microsecond',
		status=CASE WHEN $5 THEN 'failed' ELSE 'pending' END,
		failed_at=CASE WHEN $5 THEN clock_timestamp() ELSE NULL END
		WHERE id=$1 AND status='pending' AND attempts=$2`,
		id, attempts, err.Error(), w.config.retryDelay(attempts+1).Microseconds(), terminal)
	if updateErr != nil {
		return true, fmt.Errorf("persist failure for payload %d (%v): %w", id, err, updateErr)
	}
	slog.ErrorContext(ctx, "ingestion payload processing failed", "payload_id", id, "scraper_id", scraperID,
		"submission_id", submissionID, "attempt", attempts+1, "terminal", terminal, "error", err)
	return true, nil
}
