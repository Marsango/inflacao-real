package dailysummary

import (
	"context"
	"database/sql"
	"errors"
	"inflacao-real/core/internal/insights"
	"log/slog"
	"time"
)

type pendingDay struct {
	date       time.Time
	generation int64
}

func Refresh(requestContext context.Context, database *sql.DB) error {
	if err := initialize(requestContext, database); err != nil {
		return err
	}
	updated, err := refreshDays(requestContext, database)
	if err != nil || updated == 0 {
		return err
	}
	// Backfills can otherwise leave an empty-table estimate on the first read.
	_, err = database.ExecContext(requestContext, `ANALYZE history_daily`)
	return err
}

func refreshDays(requestContext context.Context, database *sql.DB) (int, error) {
	more := true
	updated := 0
	var err error
	for updated < 10 && more && err == nil {
		more, err = refreshDay(requestContext, database)
		updated += boolCount(more)
	}
	return updated, err
}

func boolCount(value bool) int {
	if value {
		return 1
	}
	return 0
}

func initialize(requestContext context.Context, database *sql.DB) error {
	transaction, err := database.BeginTx(requestContext, nil)
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	return initializeTransaction(requestContext, transaction)
}

func initializeTransaction(requestContext context.Context, transaction *sql.Tx) error {
	var initialized bool
	var methodology string
	row := transaction.QueryRowContext(requestContext, `SELECT initialized,methodology FROM history_daily_state FOR UPDATE`)
	if err := row.Scan(&initialized, &methodology); err != nil {
		return err
	}
	if initialized && methodology == insights.MethodologyVersion {
		return transaction.Commit()
	}
	if err := bootstrap(requestContext, transaction); err != nil {
		return err
	}
	return transaction.Commit()
}

func bootstrap(requestContext context.Context, transaction *sql.Tx) error {
	_, err := transaction.ExecContext(requestContext, `UPDATE history_daily_days SET generation=generation+1`)
	if err != nil {
		return err
	}
	_, err = transaction.ExecContext(requestContext, `INSERT INTO history_daily_days(day)
 SELECT DISTINCT (observed_at AT TIME ZONE 'America/Sao_Paulo')::date FROM price_observations ON CONFLICT DO NOTHING`)
	if err != nil {
		return err
	}
	_, err = transaction.ExecContext(requestContext, `UPDATE history_daily_state SET initialized=true,methodology=$1`, insights.MethodologyVersion)
	return err
}

func refreshDay(requestContext context.Context, database *sql.DB) (bool, error) {
	transaction, err := database.BeginTx(requestContext, nil)
	if err != nil {
		return false, err
	}
	defer transaction.Rollback()
	var locked bool
	if err = transaction.QueryRowContext(requestContext, `SELECT pg_try_advisory_xact_lock(71523144)`).Scan(&locked); err != nil {
		return false, err
	}
	if !locked {
		return false, nil
	}
	return rebuildNext(requestContext, transaction)
}

func rebuildNext(requestContext context.Context, transaction *sql.Tx) (bool, error) {
	var day pendingDay
	row := transaction.QueryRowContext(requestContext, `SELECT day,generation FROM history_daily_days
 WHERE generation<>summarized_generation ORDER BY day DESC LIMIT 1`)
	err := row.Scan(&day.date, &day.generation)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return day.rebuild(requestContext, transaction)
}

func (day pendingDay) rebuild(requestContext context.Context, transaction *sql.Tx) (bool, error) {
	started := time.Now()
	if err := rebuild(requestContext, transaction, day.date); err != nil {
		return false, err
	}
	published, err := day.publish(requestContext, transaction)
	if err != nil || !published {
		return published, err
	}
	slog.InfoContext(requestContext, "daily history summarized", "day", day.date.Format(time.DateOnly),
		"generation", day.generation, "duration_ms", time.Since(started).Milliseconds())
	return true, nil
}

// Publication is conditional: ingestion never waits behind a day-long lock. If
// new data committed while we calculated, this transaction is rolled back.
func (day pendingDay) publish(requestContext context.Context, transaction *sql.Tx) (bool, error) {
	result, err := transaction.ExecContext(requestContext, `UPDATE history_daily_days SET summarized_generation=$2,
 refreshed_at=now() WHERE day=$1 AND generation=$2`, day.date, day.generation)
	if err != nil {
		return false, err
	}
	updated, err := result.RowsAffected()
	if err != nil || updated == 0 {
		return false, err
	}
	if err := transaction.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func rebuild(requestContext context.Context, transaction *sql.Tx, day time.Time) error {
	if _, err := transaction.ExecContext(requestContext, `SET LOCAL jit=off`); err != nil {
		return err
	}
	if _, err := transaction.ExecContext(requestContext, `DELETE FROM history_daily WHERE day=$1`, day); err != nil {
		return err
	}
	_, err := transaction.ExecContext(requestContext, `INSERT INTO history_daily WITH source_days AS (SELECT $1::date AS day), `+aggregation+` SELECT * FROM daily_rows`, day)
	return err
}
