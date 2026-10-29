package monitoring

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

type Queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

type PostgresStore struct{ DB Queryer }

func (store PostgresStore) List(requestContext context.Context) ([]Target, error) {
	rows, err := store.DB.QueryContext(requestContext, `SELECT id,name,criteria FROM monitoring_targets WHERE enabled ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return readTargets(rows)
}

func readTargets(rows *sql.Rows) ([]Target, error) {
	targets := make([]Target, 0)
	var err error
	for rows.Next() && err == nil {
		targets, err = appendTarget(rows, targets)
	}
	if err != nil {
		return nil, err
	}
	return targets, rows.Err()
}

func appendTarget(rows *sql.Rows, targets []Target) ([]Target, error) {
	target, err := readTarget(rows)
	if err != nil {
		return nil, err
	}
	return append(targets, target), nil
}

func readTarget(rows *sql.Rows) (Target, error) {
	var target Target
	var criteria []byte
	if err := rows.Scan(&target.ID, &target.Name, &criteria); err != nil {
		return target, err
	}
	if err := json.Unmarshal(criteria, &target); err != nil {
		return target, fmt.Errorf("decode monitoring target %s: %w", target.ID, err)
	}
	return target, nil
}

func Associate(requestContext context.Context, transaction *sql.Tx, targets []Target, productID int64, name string) error {
	var err error
	for position := 0; position < len(targets) && err == nil; position++ {
		err = associateTarget(requestContext, transaction, targets[position], productID, name)
	}
	return err
}

func associateTarget(requestContext context.Context, transaction *sql.Tx, target Target, productID int64, name string) error {
	if !target.Matches(name) {
		return nil
	}
	_, err := transaction.ExecContext(requestContext, `INSERT INTO product_monitoring_targets (product_id,target_id)
		VALUES ($1,$2) ON CONFLICT DO NOTHING`, productID, target.ID)
	return err
}
