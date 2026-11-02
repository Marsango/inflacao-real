package historyprecompute

import (
	"context"
	"log/slog"
	"time"
)

type growth struct {
	observations, observationBytes, dailyRows, dailyBytes, dirtyDays int64
}

func (store *Store) logGrowth(requestContext context.Context, selections int, duration time.Duration) {
	metrics, err := store.growth(requestContext)
	if err != nil {
		slog.WarnContext(requestContext, "read history growth metrics", "error", err)
		return
	}
	slog.InfoContext(requestContext, "history precomputation", "selections", selections,
		"duration_ms", duration.Milliseconds(), "estimated_observations", metrics.observations,
		"observation_bytes", metrics.observationBytes, "estimated_daily_rows", metrics.dailyRows,
		"daily_summary_bytes", metrics.dailyBytes, "dirty_days", metrics.dirtyDays)
}

func (store *Store) growth(requestContext context.Context) (growth, error) {
	var metrics growth
	row := store.database.QueryRowContext(requestContext, `SELECT
 coalesce((SELECT n_live_tup FROM pg_stat_user_tables WHERE relname='price_observations'),0),
 pg_total_relation_size('price_observations'),
 coalesce((SELECT n_live_tup FROM pg_stat_user_tables WHERE relname='history_daily'),0),
 pg_total_relation_size('history_daily'),
 (SELECT count(*) FROM history_daily_days WHERE generation<>summarized_generation)`)
	err := row.Scan(&metrics.observations, &metrics.observationBytes, &metrics.dailyRows, &metrics.dailyBytes, &metrics.dirtyDays)
	return metrics, err
}
