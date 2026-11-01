package historycache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"golang.org/x/sync/singleflight"

	"inflacao-real/core/internal/insights"
	"inflacao-real/core/internal/webapi"
)

type source interface {
	webapi.Store
	Revision(context.Context) (int64, error)
}

type Store struct {
	source    source
	lifecycle context.Context
	cache     *memory
	flights   singleflight.Group
	now       func() time.Time
}

func New(lifecycle context.Context, underlying source, configuration Config) (*Store, error) {
	if configuration.TTL <= 0 || configuration.MaxEntries <= 0 || configuration.MaxBytes <= 0 {
		return nil, fmt.Errorf("history cache limits must be positive")
	}
	return &Store{source: underlying, lifecycle: lifecycle, cache: newMemory(configuration), now: time.Now}, nil
}

func (store *Store) Products(requestContext context.Context, request webapi.ProductRequest) (webapi.Products, error) {
	underlying := store.source
	return underlying.Products(requestContext, request)
}

func (store *Store) History(requestContext context.Context, request webapi.HistoryRequest) (insights.History, error) {
	if err := requestContext.Err(); err != nil {
		return insights.History{}, err
	}
	underlying, cache := store.source, store.cache
	revision, err := underlying.Revision(requestContext)
	if err != nil {
		return insights.History{}, err
	}
	key := cacheKey{Request: request, Revision: revision, Methodology: insights.MethodologyVersion}
	data, found := cache.lookup(key, store.now())
	cache.recordRequest(found)
	if found {
		return decodeHistory(data, store.now())
	}
	return store.wait(requestContext, key)
}

func (store *Store) wait(requestContext context.Context, key cacheKey) (insights.History, error) {
	identity, _ := json.Marshal(key)
	flights := &store.flights
	result := flights.DoChan(string(identity), func() (any, error) { return store.load(requestContext, key) })
	select {
	case <-requestContext.Done():
		return insights.History{}, requestContext.Err()
	case loaded := <-result:
		return decodeResult(loaded, store.now())
	}
}

func decodeResult(result singleflight.Result, now time.Time) (insights.History, error) {
	if result.Err != nil {
		return insights.History{}, result.Err
	}
	return decodeHistory(result.Val.([]byte), now)
}

// Decoding isolates nested slices and pointers from both the cache and other callers.
func decodeHistory(data []byte, now time.Time) (insights.History, error) {
	var history insights.History
	err := json.Unmarshal(data, &history)
	history.RefreshFreshness(now)
	return history, err
}
