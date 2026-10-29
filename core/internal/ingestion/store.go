package ingestion

import (
	"context"
	"database/sql"
	"fmt"

	"inflacao-real/core/internal/ingestion/document"
)

type Submission = document.Submission

type Store interface {
	Enqueue(context.Context, Submission) (bool, error)
}

type PostgresStore struct{ DB *sql.DB }

const enqueueSQL = `INSERT INTO ingestion_payloads
    (scraper_id, submission_id, shop_id, schema_version, created_at, payload)
    VALUES ($1, $2, $3, $4, $5::timestamptz, $6::jsonb)
    ON CONFLICT (scraper_id, submission_id) DO NOTHING`

// ExecContext finishes the implicit transaction before reporting acceptance.
func (store PostgresStore) Enqueue(requestContext context.Context, submission Submission) (bool, error) {
	result, err := store.DB.ExecContext(requestContext, enqueueSQL, submission.ScraperID,
		submission.SubmissionID, submission.ShopID, submission.SchemaVersion, submission.CreatedAt,
		string(submission.Payload))
	if err != nil {
		return false, fmt.Errorf("enqueue ingestion payload: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read ingestion insert result: %w", err)
	}
	return rows == 1, nil
}
