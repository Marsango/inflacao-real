package dailysummary

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"inflacao-real/core/internal/insights"
)

func History(requestContext context.Context, transaction *sql.Tx, start, end string, targets []string) (insights.History, error) {
	rows, err := Overview(requestContext, transaction, start, end, targets)
	if err != nil {
		return insights.History{}, err
	}
	defer rows.Close()
	calculator, err := readOverview(rows)
	if err != nil {
		return insights.History{}, err
	}
	return calculator.Build(start, end, time.Now()), nil
}

func readOverview(rows *sql.Rows) (*insights.OverviewCalculator, error) {
	if !rows.Next() {
		return nil, sql.ErrNoRows
	}
	calculator, err := readMetadata(rows)
	if err != nil {
		return nil, err
	}
	for rows.Next() && err == nil {
		err = readTerm(rows, calculator)
	}
	if err != nil {
		return nil, err
	}
	return calculator, rows.Err()
}

func readMetadata(rows *sql.Rows) (*insights.OverviewCalculator, error) {
	var kind, streams int
	var day, denominator, numerator sql.NullString
	var data []byte
	err := rows.Scan(&kind, &day, &denominator, &numerator, &streams, &data)
	if err != nil {
		return nil, err
	}
	var metadata insights.OverviewMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}
	return insights.NewOverview(metadata), nil
}

func readTerm(rows *sql.Rows, calculator *insights.OverviewCalculator) error {
	var kind, streams int
	var day, denominator, numerator sql.NullString
	var data []byte
	if err := rows.Scan(&kind, &day, &denominator, &numerator, &streams, &data); err != nil {
		return err
	}
	if streams == 0 {
		return nil
	}
	return calculator.Add(day.String, numerator.String, denominator.String, streams)
}
