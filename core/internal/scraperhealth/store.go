package scraperhealth

import (
	"context"
	"database/sql"
)

func Save(ctx context.Context, database *sql.DB, report Report) (bool, error) {
	transaction, err := database.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer transaction.Rollback()
	return persistRun(ctx, transaction, report)
}
func persistRun(ctx context.Context, transaction *sql.Tx, report Report) (bool, error) {
	result, err := insertRun(ctx, transaction, report)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if count == 0 {
		return false, nil
	}
	if err = saveDiagnostics(ctx, transaction, report); err != nil {
		return false, err
	}
	return true, transaction.Commit()
}

func saveDiagnostics(ctx context.Context, transaction *sql.Tx, report Report) error {
	var err error
	for index := 0; index < len(report.Diagnostics) && err == nil; index++ {
		err = saveDiagnostic(ctx, transaction, report, report.Diagnostics[index])
	}
	return err
}
func saveDiagnostic(ctx context.Context, transaction *sql.Tx, report Report, diagnostic Diagnostic) error {
	_, err := transaction.ExecContext(ctx, `INSERT INTO scraper_diagnostics(scraper_id,shop_id,run_id,severity,category,message) VALUES($1,$2,$3,$4,$5,$6)`, report.ScraperID, report.ShopID, report.RunID, diagnostic.Severity, diagnostic.Category, diagnostic.Message)
	return err
}

func insertRun(ctx context.Context, transaction *sql.Tx, report Report) (sql.Result, error) {
	return transaction.ExecContext(ctx, `INSERT INTO scraper_runs
 (scraper_id,shop_id,run_id,scraper_name,started_at,finished_at,observations,errors,warnings,omitted)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT DO NOTHING`,
		report.ScraperID, report.ShopID, report.RunID, report.Name, report.Started, report.Finished, report.Observations, report.Errors, report.Warnings, report.Omitted)
}
