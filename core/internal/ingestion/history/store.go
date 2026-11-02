package history

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"

	"inflacao-real/core/internal/ingestion/document"
)

//go:embed unchanged.sql
var unchangedQuery string

// SkipUnchanged runs under the ingestion transaction's catalog lock. Independent
// collectors and shopping contexts are intentionally compared separately.
func SkipUnchanged(requestContext context.Context, transaction *sql.Tx, listingID, productID int64, scraperID string, observation document.Observation) (bool, error) {
	states, err := json.Marshal(measurementStates(observation, productID))
	if err != nil {
		return false, err
	}
	result, err := transaction.ExecContext(requestContext, unchangedQuery,
		listingID, scraperID, string(states), observation.ObservedAt)
	if err != nil {
		return false, err
	}
	updated, err := result.RowsAffected()
	return updated > 0, err
}
