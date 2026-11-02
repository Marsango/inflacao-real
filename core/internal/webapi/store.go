package webapi

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"time"

	"inflacao-real/core/internal/dailysummary"
	"inflacao-real/core/internal/insights"
	"inflacao-real/core/internal/selections"
)

//go:embed history.sql
var historyQuery string

//go:embed products.sql
var productsQuery string

type PostgresStore struct{ DB *sql.DB }

// Revision advances atomically with ingestion processing and equivalence decisions.
func (store PostgresStore) Revision(requestContext context.Context) (int64, error) {
	var revision int64
	row := store.DB.QueryRowContext(requestContext, `SELECT revision FROM catalog_decisions`)
	err := row.Scan(&revision)
	return revision, err
}

func (store PostgresStore) Products(requestContext context.Context, request ProductRequest) (Products, error) {
	rows, err := store.DB.QueryContext(requestContext, productsQuery, request.After, request.Limit+1, request.Query, selections.Targets(request.Group))
	if err != nil {
		return Products{}, err
	}
	defer rows.Close()
	return readProducts(rows, request.Limit)
}

func readProducts(rows *sql.Rows, limit int) (Products, error) {
	result := Products{Items: []Product{}}
	var err error
	for rows.Next() && err == nil {
		result.Items, err = appendProduct(rows, result.Items)
	}
	if err != nil {
		return result, err
	}
	if len(result.Items) > limit {
		result.NextCursor = &result.Items[limit-1].ID
		result.Items = result.Items[:limit]
	}
	return result, rows.Err()
}

func appendProduct(rows *sql.Rows, products []Product) ([]Product, error) {
	var product Product
	if err := rows.Scan(&product.ID, &product.Label); err != nil {
		return products, err
	}
	return append(products, product), nil
}

func (store PostgresStore) History(requestContext context.Context, request HistoryRequest) (insights.History, error) {
	transaction, err := store.DB.BeginTx(requestContext, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return insights.History{}, err
	}
	defer transaction.Rollback()
	result, err := storedHistory(requestContext, transaction, request)
	if err != nil {
		return result, err
	}
	return result, transaction.Commit()
}
func storedHistory(requestContext context.Context, transaction *sql.Tx, request HistoryRequest) (insights.History, error) {
	if _, err := transaction.ExecContext(requestContext, `SET LOCAL jit = off`); err != nil {
		return insights.History{}, err
	}
	name, productID, err := resolveProduct(requestContext, transaction, request.Product)
	if err != nil {
		return insights.History{}, err
	}
	if request.Product == "global" {
		return storedOverview(requestContext, transaction, request)
	}
	rows, err := transaction.QueryContext(requestContext, historyQuery, productID, request.Start, request.End, selections.Targets(request.Group), false)
	if err != nil {
		return insights.History{}, err
	}
	defer rows.Close()
	return buildHistory(rows, request, name)
}

func buildHistory(rows *sql.Rows, request HistoryRequest, name string) (insights.History, error) {
	series, shops, err := readSeries(rows)
	if err != nil {
		return insights.History{}, err
	}
	series = insights.ExpandSparse(series, request.Start, request.End)
	global := request.Product == "global"
	result := insights.Build(series, global, shops, time.Now())
	result.Period = insights.Period{Start: request.Start, End: request.End}
	if !global {
		result.Title, result.Unit = name, "BRL"
	}
	result = describeSelection(result, request)
	return result, nil
}

func describeSelection(history insights.History, request HistoryRequest) insights.History {
	group, found := selections.Find(request.Group)
	if !found {
		return history
	}
	history.Methodology += " Seleção: " + group.Label + ". " + group.Description + " A composição atual dos rótulos é aplicada ao período consultado; produtos sem classificação ficam fora deste filtro."
	if request.Product == "global" {
		history.Title = "Variação de preços · " + group.Label
	}
	return history
}

func resolveProduct(requestContext context.Context, transaction *sql.Tx, value string) (string, any, error) {
	if value == "global" {
		return "", nil, nil
	}
	productID, err := identity(value)
	if err != nil {
		return "", nil, ErrNotFound
	}
	var name string
	row := transaction.QueryRowContext(requestContext, `SELECT name,id FROM products WHERE id=equivalent_product_id($1)`, productID)
	err = row.Scan(&name, &productID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil, ErrNotFound
	}
	return name, productID, err
}

func readSeries(rows *sql.Rows) ([]insights.ProductSeries, int, error) {
	result := []insights.ProductSeries{}
	shops := 0
	var err error
	for rows.Next() && err == nil {
		result, shops, err = appendSeries(rows, result)
	}
	if err != nil {
		return nil, 0, err
	}
	return result, shops, rows.Err()
}

func appendSeries(rows *sql.Rows, series []insights.ProductSeries) ([]insights.ProductSeries, int, error) {
	product, shops, err := readProductSeries(rows)
	return append(series, product), shops, err
}

func readProductSeries(rows *sql.Rows) (insights.ProductSeries, int, error) {
	var product insights.ProductSeries
	var points []byte
	var shops int
	err := rows.Scan(&product.ID, &product.Name, &product.PriceBasis, &product.UnknownConditions, &product.CollectedAt, &product.Streams, &product.Shops, &shops, &points)
	if err != nil {
		return product, 0, err
	}
	err = json.Unmarshal(points, &product.Contexts)
	return product, shops, err
}

func storedOverview(requestContext context.Context, transaction *sql.Tx, request HistoryRequest) (insights.History, error) {
	initialized, err := dailysummary.Initialized(requestContext, transaction)
	if err != nil {
		return insights.History{}, err
	}
	if initialized {
		return summarizedOverview(requestContext, transaction, request)
	}
	rows, err := transaction.QueryContext(requestContext, historyQuery, nil, request.Start, request.End, selections.Targets(request.Group), true)
	if err != nil {
		return insights.History{}, err
	}
	defer rows.Close()
	return buildHistory(rows, request, "")
}

func summarizedOverview(requestContext context.Context, transaction *sql.Tx, request HistoryRequest) (insights.History, error) {
	history, err := dailysummary.History(requestContext, transaction, request.Start, request.End, selections.Targets(request.Group))
	if err != nil {
		return history, err
	}
	history.Period = insights.Period{Start: request.Start, End: request.End}
	return describeSelection(history, request), nil
}
