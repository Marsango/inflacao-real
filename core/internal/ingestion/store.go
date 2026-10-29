package ingestion

import (
	"context"
	"database/sql"
	"fmt"
)

// Submission contains validated queue metadata and the complete original JSON.
// Money remains in the payload as decimal strings and is never parsed as float.
type Submission struct {
	ScraperID     string
	SubmissionID  string
	ShopID        string
	SchemaVersion string
	CreatedAt     string
	Payload       []byte
}

// Store returns true only when a new submission has committed durably.
type Store interface {
	Enqueue(context.Context, Submission) (bool, error)
}

type PostgresStore struct {
	DB *sql.DB
}

func (s PostgresStore) Enqueue(ctx context.Context, submission Submission) (bool, error) {
	// A single statement is an implicit transaction. ExecContext returns after
	// commit; the unique constraint serializes concurrent submissions/retries.
	// Database defaults supply pending status, receipt/retry times and attempts.
	result, err := s.DB.ExecContext(ctx, `INSERT INTO ingestion_payloads
		(scraper_id, submission_id, shop_id, schema_version, created_at, payload)
		VALUES ($1, $2, $3, $4, $5::timestamptz, $6::jsonb)
		ON CONFLICT (scraper_id, submission_id) DO NOTHING`,
		submission.ScraperID, submission.SubmissionID, submission.ShopID,
		submission.SchemaVersion, submission.CreatedAt, string(submission.Payload))
	if err != nil {
		return false, fmt.Errorf("enqueue ingestion payload: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read ingestion insert result: %w", err)
	}
	return rows == 1, nil
}
