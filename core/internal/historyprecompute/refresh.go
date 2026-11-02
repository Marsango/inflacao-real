package historyprecompute

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"log/slog"
	"time"

	"inflacao-real/core/internal/dailysummary"
	"inflacao-real/core/internal/insights"
	"inflacao-real/core/internal/selections"
	"inflacao-real/core/internal/webapi"
)

func (store *Store) Run(requestContext context.Context) {
	timer := time.NewTicker(30 * time.Second)
	defer timer.Stop()
	for requestContext.Err() == nil {
		store.refreshCycle(requestContext)
		store.wait(requestContext, timer.C)
	}
}

func (store *Store) refreshCycle(requestContext context.Context) {
	cycle, cancel := context.WithTimeout(requestContext, 2*time.Minute)
	defer cancel()
	if err := store.Refresh(cycle, time.Now()); err != nil && requestContext.Err() == nil {
		slog.ErrorContext(requestContext, "history precomputation failed", "error", err)
	}
}

// Refresh uses a dedicated connection to hold the advisory lock across calculations.
func (store *Store) Refresh(requestContext context.Context, now time.Time) error {
	connection, err := store.database.Conn(requestContext)
	if err != nil {
		return err
	}
	defer connection.Close()
	var locked bool
	if err := connection.QueryRowContext(requestContext, `SELECT pg_try_advisory_lock(71523143)`).Scan(&locked); err != nil {
		return err
	}
	if !locked {
		return nil
	}
	defer releaseLock(connection)
	return store.refreshReady(requestContext, now)
}

func releaseLock(connection *sql.Conn) {
	requestContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := connection.ExecContext(requestContext, `SELECT pg_advisory_unlock(71523143)`); err != nil {
		// A connection with an uncertain session lock must not return to the pool.
		connection.Raw(func(any) error { return driver.ErrBadConn })
	}
}

func (store *Store) refreshReady(requestContext context.Context, now time.Time) error {
	ready, err := store.ready(requestContext)
	if err != nil {
		return err
	}
	if !ready {
		return nil
	}
	if err := store.prepare(requestContext, now); err != nil {
		return err
	}
	revision, err := store.Revision(requestContext)
	if err != nil {
		return err
	}
	if err := dailysummary.Refresh(requestContext, store.database); err != nil {
		return err
	}
	return store.refreshRevision(requestContext, revision)
}

func (store *Store) seedOverview(requestContext context.Context, now time.Time) error {
	location, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		return err
	}
	today := now.In(location)
	first := previousMonth(today)
	groups := append([]selections.Group{{}}, selections.All()...)
	var failure error
	for position := 0; position < len(groups) && failure == nil; position++ {
		group := groups[position]
		request := webapi.HistoryRequest{Product: "global", Start: first.Format(time.DateOnly), End: today.Format(time.DateOnly), Group: group.ID}
		failure = store.seed(requestContext, request)
	}
	return failure
}

func (store *Store) staleRequests(requestContext context.Context, revision int64) ([]webapi.HistoryRequest, error) {
	rows, err := store.database.QueryContext(requestContext, `SELECT request FROM history_precomputations
  WHERE methodology=$1 AND (revision<>$2 OR result IS NULL)
  ORDER BY requested_at DESC,identity LIMIT 10`, insights.MethodologyVersion, revision)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return readRequests(rows)
}

func readRequests(rows *sql.Rows) ([]webapi.HistoryRequest, error) {
	requests := []webapi.HistoryRequest{}
	var err error
	for rows.Next() && err == nil {
		requests, err = appendRequest(rows, requests)
	}
	if err != nil {
		return nil, err
	}
	return requests, rows.Err()
}

func appendRequest(rows *sql.Rows, requests []webapi.HistoryRequest) ([]webapi.HistoryRequest, error) {
	var data []byte
	var request webapi.HistoryRequest
	if err := rows.Scan(&data); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &request); err != nil {
		return nil, err
	}
	return append(requests, request), nil
}

func (store *Store) refreshSelections(requestContext context.Context, requests []webapi.HistoryRequest, revision int64) error {
	started := time.Now()
	var err error
	current := true
	for position := 0; position < len(requests) && current && err == nil; position++ {
		current, err = store.refreshCurrent(requestContext, requests[position], revision)
	}
	if err != nil {
		return err
	}
	if !current {
		return nil
	}

	if len(requests) > 0 {
		store.logGrowth(requestContext, len(requests), time.Since(started))
	}
	return store.complete(requestContext, revision, started)
}

func (store *Store) current(requestContext context.Context, revision int64) (bool, error) {
	var current bool
	err := store.database.QueryRowContext(requestContext, `SELECT revision=$1 AND NOT EXISTS(
  SELECT 1 FROM ingestion_payloads WHERE status='pending') FROM catalog_decisions`, revision).Scan(&current)
	return current, err
}

func (store *Store) complete(requestContext context.Context, revision int64, started time.Time) error {
	_, err := store.database.ExecContext(requestContext, `UPDATE history_refresh_requests SET
  completed_generation=generation,completed_at=now() WHERE requested_at<=$3 AND NOT EXISTS (
  SELECT 1 FROM history_precomputations WHERE methodology=$1 AND (revision<>$2 OR result IS NULL))
  AND NOT EXISTS(SELECT 1 FROM ingestion_payloads WHERE status='pending')
  AND (SELECT revision FROM catalog_decisions)=$2`, insights.MethodologyVersion, revision, started)
	return err
}

func (store *Store) notify() {
	select {
	case store.wake <- struct{}{}:
	default:
	}
}

func (store *Store) prepare(requestContext context.Context, now time.Time) error {
	if err := store.prune(requestContext); err != nil {
		return err
	}
	return store.seedOverview(requestContext, now)
}

// Match the web default: the same local day in the preceding month, clamped
// to the last day of that month (for example March 31 becomes February 28).
func previousMonth(today time.Time) time.Time {
	location := today.Location()
	first := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, location)
	last := first.AddDate(0, 0, -1)
	return time.Date(last.Year(), last.Month(), min(today.Day(), last.Day()), 0, 0, 0, 0, location)
}

func (store *Store) refreshRevision(requestContext context.Context, revision int64) error {
	requests, err := store.staleRequests(requestContext, revision)
	if err != nil {
		return err
	}
	return store.refreshSelections(requestContext, requests, revision)
}

func (store *Store) refreshSelection(requestContext context.Context, request webapi.HistoryRequest, revision int64) {
	if _, err := store.calculate(requestContext, request, revision, false); err != nil {
		slog.ErrorContext(requestContext, "history selection precomputation failed", "product", request.Product,
			"start", request.Start, "end", request.End, "group", request.Group, "error", err)
	}
}

func (store *Store) refreshCurrent(requestContext context.Context, request webapi.HistoryRequest, revision int64) (bool, error) {
	current, err := store.current(requestContext, revision)
	if err != nil || !current {
		return current, err
	}
	store.refreshSelection(requestContext, request, revision)
	return true, nil
}

func (store *Store) wait(requestContext context.Context, ticks <-chan time.Time) {
	select {
	case <-requestContext.Done():
	case <-ticks:
	case <-store.wake:
	}
}

func (store *Store) ready(requestContext context.Context) (bool, error) {
	var ready bool
	err := store.database.QueryRowContext(requestContext, `SELECT NOT EXISTS(
  SELECT 1 FROM ingestion_payloads WHERE status='pending') AND (
  EXISTS(SELECT 1 FROM history_refresh_requests WHERE generation>completed_generation)
  OR NOT EXISTS(SELECT 1 FROM ingestion_payloads WHERE processed_at>now()-interval '2 minutes'))`).Scan(&ready)
	return ready, err
}
