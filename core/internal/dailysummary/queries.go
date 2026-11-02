// Package dailysummary maintains reusable daily context measurements. Raw IDs and
// separate known/unknown price candidates preserve interval-dependent policy.
package dailysummary

import (
	"context"
	"database/sql"
	_ "embed"
	"inflacao-real/core/internal/insights"
)

//go:embed aggregate.sql
var aggregation string

//go:embed measurements.sql
var measurements string

//go:embed overview.sql
var overview string

const dirtyDays = `WITH source_days AS (SELECT day FROM history_daily_days
 WHERE day BETWEEN $2::date AND $3::date AND generation<>summarized_generation), `

const cleanDays = `WITH source_days AS (SELECT NULL::date AS day WHERE false), `

// Initialized is read in the caller's history snapshot. Until bootstrap finishes,
// readers use the existing raw path, so an unbuilt summary never appears empty.
func Initialized(requestContext context.Context, transaction *sql.Tx) (bool, error) {
	var initialized bool
	err := transaction.QueryRowContext(requestContext, `SELECT initialized AND methodology=$1 FROM history_daily_state`, insights.MethodologyVersion).Scan(&initialized)
	return initialized, err
}

func Overview(requestContext context.Context, transaction *sql.Tx, start, end string, targets []string) (*sql.Rows, error) {
	source, err := overviewDays(requestContext, transaction, start, end)
	if err != nil {
		return nil, err
	}
	return transaction.QueryContext(requestContext, source+aggregation+measurements+overview, nil, start, end, targets)
}

// A known-empty raw branch avoids severe cardinality underestimates when every
// day is clean. Read the generations in the same snapshot as the history query.
func overviewDays(requestContext context.Context, transaction *sql.Tx, start, end string) (string, error) {
	var dirty bool
	row := transaction.QueryRowContext(requestContext, `SELECT EXISTS(SELECT 1 FROM history_daily_days
 WHERE day BETWEEN $1::date AND $2::date AND generation<>summarized_generation)`, start, end)
	if err := row.Scan(&dirty); err != nil {
		return "", err
	}
	if dirty {
		return dirtyDays, nil
	}
	return cleanDays, nil
}

// MarkPayload is committed atomically with the raw observations and revision.
func MarkPayload(requestContext context.Context, transaction *sql.Tx, identity int64) error {
	_, err := transaction.ExecContext(requestContext, `INSERT INTO history_daily_days(day)
 SELECT DISTINCT ((observation->>'observed_at')::timestamptz AT TIME ZONE 'America/Sao_Paulo')::date
 FROM ingestion_payloads CROSS JOIN LATERAL jsonb_array_elements(payload->'observations') observation
 WHERE id=$1
 ON CONFLICT(day) DO UPDATE SET generation=history_daily_days.generation+1`, identity)
	return err
}

func Query() string { return dirtyDays + aggregation + measurements + overview }
