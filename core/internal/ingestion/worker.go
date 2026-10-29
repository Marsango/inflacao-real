package ingestion

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"inflacao-real/core/internal/ingestion/document"
)

type Worker struct {
	database *sql.DB
	config   WorkerConfig
	schema   *jsonschema.Schema
}

func NewWorker(database *sql.DB, configuration WorkerConfig) (*Worker, error) {
	if database == nil {
		return nil, errors.New("worker requires a database")
	}
	if err := configuration.validate(); err != nil {
		return nil, err
	}
	schema, err := document.CompileSchema()
	if err != nil {
		return nil, fmt.Errorf("compile worker schema: %w", err)
	}
	return &Worker{database: database, config: configuration, schema: schema}, nil
}

// Run must be canceled and joined before closing the database pool.
func (worker *Worker) Run(requestContext context.Context) {
	runLoop(requestContext, worker.config.BatchSize, worker.ProcessNext, worker.idle)
}

func (worker *Worker) idle(requestContext context.Context) {
	worker.logQueue(requestContext)
	timer := time.NewTimer(worker.config.PollInterval)
	defer timer.Stop()
	select {
	case <-requestContext.Done():
	case <-timer.C:
	}
}

func (worker *Worker) logQueue(requestContext context.Context) {
	statsContext, cancel := context.WithTimeout(requestContext, 10*time.Second)
	defer cancel()
	var backlog, failed, retries int64
	var oldest float64
	row := worker.database.QueryRowContext(statsContext, queueMetricsSQL)
	err := row.Scan(&backlog, &oldest, &failed, &retries)
	if err != nil {
		logCycleFailure(requestContext, "read ingestion queue metrics", err)
		return
	}
	slog.InfoContext(requestContext, "ingestion queue", "pending", backlog,
		"oldest_pending_seconds", oldest, "failed", failed, "retries", retries)
}

func runLoop(requestContext context.Context, batchSize int, process func(context.Context) (bool, error), idle func(context.Context)) {
	for requestContext.Err() == nil {
		runCycle(requestContext, batchSize, process, idle)
	}
}

func runCycle(requestContext context.Context, batchSize int, process func(context.Context) (bool, error), idle func(context.Context)) {
	cycle := processingCycle{ready: true}
	for cycle.ready && cycle.processed < batchSize && requestContext.Err() == nil {
		cycle.step(requestContext, process)
	}
	if requestContext.Err() == nil && cycle.processed < batchSize {
		idle(requestContext)
	}
}

type processingCycle struct {
	ready     bool
	processed int
}

func (cycle *processingCycle) step(requestContext context.Context, process func(context.Context) (bool, error)) {
	found, err := process(requestContext)
	if err != nil {
		logCycleFailure(requestContext, "ingestion worker cycle failed", err)
		cycle.ready = false
		return
	}
	if !found {
		cycle.ready = false
		return
	}
	cycle.processed++
}

func logCycleFailure(requestContext context.Context, message string, err error) {
	if requestContext.Err() == nil {
		slog.ErrorContext(requestContext, message, "error", err)
	}
}

func (worker *Worker) ProcessNext(requestContext context.Context) (bool, error) {
	processingContext, cancel := context.WithTimeout(requestContext, 10*time.Second)
	defer cancel()
	transaction, err := worker.database.BeginTx(processingContext, nil)
	if err != nil {
		return false, fmt.Errorf("begin payload processing: %w", err)
	}
	defer transaction.Rollback()
	return worker.processTransaction(requestContext, processingContext, transaction)
}
