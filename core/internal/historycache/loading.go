package historycache

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"
)

func (store *Store) load(requestContext context.Context, key cacheKey) ([]byte, error) {
	cache := store.cache
	if data, found := cache.lookup(key, store.now()); found {
		return data, nil
	}
	// Shared work retains request values, with its own deadline and shutdown cancellation.
	calculationContext, cancel := context.WithTimeout(context.WithoutCancel(requestContext), 10*time.Second)
	defer cancel()
	stop := context.AfterFunc(store.lifecycle, cancel)
	defer stop()
	started := time.Now()
	data, err := store.calculate(calculationContext, key)
	cache.recordCalculation(time.Since(started), err)
	store.logCalculation(calculationContext, time.Since(started), err)
	return data, err
}

func (store *Store) calculate(requestContext context.Context, key cacheKey) ([]byte, error) {
	underlying, cache := store.source, store.cache
	history, err := underlying.History(requestContext, key.Request)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(history)
	if err != nil {
		return nil, err
	}
	cache.put(key, data, store.now())
	return data, nil
}

func (store *Store) logCalculation(requestContext context.Context, duration time.Duration, err error) {
	statistics := store.Stats()
	slog.InfoContext(requestContext, "history cache calculation", "duration_ms", duration.Milliseconds(),
		"success", err == nil, "hits", statistics.Hits, "misses", statistics.Misses,
		"calculations", statistics.Calculations, "failures", statistics.Failures,
		"entries", statistics.Entries, "bytes", statistics.Bytes, "evictions", statistics.Evictions,
		"invalidations", statistics.Invalidations)
}
