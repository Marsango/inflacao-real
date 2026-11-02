package historyprecompute

import (
	"context"
	"encoding/json"

	"inflacao-real/core/internal/insights"
	"inflacao-real/core/internal/webapi"
)

func (store *Store) lookup(requestContext context.Context, request webapi.HistoryRequest, revision int64) (insights.History, error) {
	identity, _ := selectionIdentity(request)
	var data []byte
	row := store.database.QueryRowContext(requestContext, `UPDATE history_precomputations SET requested_at=now()
  WHERE identity=$1 AND revision=$2 AND result IS NOT NULL RETURNING result`, identity, revision)
	if err := row.Scan(&data); err != nil {
		return insights.History{}, err
	}
	var history insights.History
	err := json.Unmarshal(data, &history)
	return history, err
}

func (store *Store) save(requestContext context.Context, request webapi.HistoryRequest, revision int64, history insights.History, requested bool) error {
	data, err := json.Marshal(history)
	if err != nil || len(data) > 1048576 {
		return err
	}
	identity, parameters := selectionIdentity(request)
	_, err = store.database.ExecContext(requestContext, `INSERT INTO history_precomputations
  (identity,request,methodology,revision,result,computed_at) SELECT $1,$2,$3,$4,$5,now()
  WHERE (SELECT revision FROM catalog_decisions)=$4
  ON CONFLICT (identity) DO UPDATE SET revision=excluded.revision,result=excluded.result,
   computed_at=excluded.computed_at,
   requested_at=CASE WHEN $6 THEN now() ELSE history_precomputations.requested_at END
  WHERE history_precomputations.revision <= excluded.revision`,
		identity, parameters, insights.MethodologyVersion, revision, data, requested)
	return err
}

func (store *Store) seed(requestContext context.Context, request webapi.HistoryRequest) error {
	identity, parameters := selectionIdentity(request)
	_, err := store.database.ExecContext(requestContext, `INSERT INTO history_precomputations
  (identity,request,methodology) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`,
		identity, parameters, insights.MethodologyVersion)
	return err
}

func (store *Store) prune(requestContext context.Context) error {
	_, err := store.database.ExecContext(requestContext, `DELETE FROM history_precomputations
  WHERE requested_at < now()-interval '7 days' OR methodology <> $1 OR identity IN (
  SELECT identity FROM history_precomputations ORDER BY requested_at DESC, identity OFFSET 100)`,
		insights.MethodologyVersion)
	return err
}
