// Package historyprecompute persists revision-specific histories and rebuilds
// recently used selections after committed collection data changes.
package historyprecompute

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"inflacao-real/core/internal/insights"
	"inflacao-real/core/internal/webapi"
)

type source interface {
	webapi.Store
	Revision(context.Context) (int64, error)
}

type Store struct {
	database *sql.DB
	source   source
	wake     chan struct{}
}

func New(database *sql.DB, underlying source) *Store {
	return &Store{database: database, source: underlying, wake: make(chan struct{}, 1)}
}

func (store *Store) Products(requestContext context.Context, request webapi.ProductRequest) (webapi.Products, error) {
	return store.source.Products(requestContext, request)
}

func (store *Store) Revision(requestContext context.Context) (int64, error) {
	return store.source.Revision(requestContext)
}

func (store *Store) History(requestContext context.Context, request webapi.HistoryRequest) (insights.History, error) {
	revision, err := store.Revision(requestContext)
	if err != nil {
		return insights.History{}, err
	}
	history, err := store.lookup(requestContext, request, revision)
	if err == nil {
		history.RefreshFreshness(time.Now())
		return history, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		slog.WarnContext(requestContext, "read precomputed history", "error", err)
	}
	return store.calculate(requestContext, request, revision, true)
}

func (store *Store) calculate(requestContext context.Context, request webapi.HistoryRequest, revision int64, requested bool) (insights.History, error) {
	history, err := store.source.History(requestContext, request)
	if err != nil {
		return history, err
	}
	if err := store.save(requestContext, request, revision, history, requested); err != nil {
		slog.WarnContext(requestContext, "save precomputed history", "error", err)
	}
	return history, nil
}

func selectionIdentity(request webapi.HistoryRequest) (string, []byte) {
	data, _ := json.Marshal(request)
	return insights.MethodologyVersion + ":" + string(data), data
}
