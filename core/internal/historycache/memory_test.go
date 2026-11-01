package historycache

import (
	"testing"
	"time"
)

func TestLeastRecentlyUsedEviction(test *testing.T) {
	source := &stubSource{}
	configuration := DefaultConfig()
	configuration.MaxEntries = 2
	store := newTestStore(test, source, configuration)
	readHistory(test, store, requestForProduct(1))
	readHistory(test, store, requestForProduct(2))
	readHistory(test, store, requestForProduct(1))
	readHistory(test, store, requestForProduct(3))
	readHistory(test, store, requestForProduct(1))
	assertCalls(test, source, 3)
	readHistory(test, store, requestForProduct(2))
	assertCalls(test, source, 4)
	if statistics := store.Stats(); statistics.Entries != 2 || statistics.Evictions != 2 {
		test.Fatalf("unexpected eviction statistics: %+v", statistics)
	}
}

func TestByteBudgetEvictsAndSkipsOversizedEntries(test *testing.T) {
	configuration := DefaultConfig()
	configuration.MaxBytes = 5
	cache := newMemory(configuration)
	first, second := cacheKey{Request: requestForProduct(1)}, cacheKey{Request: requestForProduct(2)}
	cache.put(first, []byte("123"), time.Now())
	cache.put(second, []byte("456"), time.Now())
	if _, found := cache.lookup(first, time.Now()); found {
		test.Fatal("byte limit did not evict the oldest entry")
	}
	cache.put(first, []byte("123456"), time.Now())
	if len(cache.entries) != 1 || cache.statistics.Bytes != 3 || cache.statistics.Evictions != 1 {
		test.Fatal("oversized entry changed the cache")
	}
}

func TestOversizedHistoryStillReturnsSuccessfully(test *testing.T) {
	source := &stubSource{}
	configuration := DefaultConfig()
	configuration.MaxBytes = 1
	store := newTestStore(test, source, configuration)
	readHistory(test, store, defaultRequest())
	readHistory(test, store, defaultRequest())
	assertCalls(test, source, 2)
	if store.Stats().Bytes != 0 || store.Stats().Entries != 0 {
		test.Fatal("oversized history retained")
	}
}

func TestMethodologySeparatesEntries(test *testing.T) {
	cache := newMemory(DefaultConfig())
	key := cacheKey{Request: defaultRequest(), Methodology: "old"}
	cache.put(key, []byte("history"), time.Now())
	key.Methodology = "new"
	if _, found := cache.lookup(key, time.Now()); found {
		test.Fatal("methodology reused a result calculated under another version")
	}
}

func TestCacheConfiguration(test *testing.T) {
	values := map[string]string{"HISTORY_CACHE_TTL": "2m", "HISTORY_CACHE_MAX_ENTRIES": "20", "HISTORY_CACHE_MAX_BYTES": "4096"}
	configuration, err := ConfigFromEnv(func(name string) string { return values[name] })
	requireSuccess(test, err)
	if configuration.TTL != 2*time.Minute || configuration.MaxEntries != 20 || configuration.MaxBytes != 4096 {
		test.Fatalf("configuration not loaded: %+v", configuration)
	}
	defaults, err := ConfigFromEnv(func(string) string { return "" })
	requireSuccess(test, err)
	if defaults != DefaultConfig() {
		test.Fatal("incorrect default configuration")
	}
}

func TestInvalidConfiguration(test *testing.T) {
	for _, name := range []string{"HISTORY_CACHE_TTL", "HISTORY_CACHE_MAX_ENTRIES", "HISTORY_CACHE_MAX_BYTES"} {
		assertInvalidSetting(test, name)
	}
	_, err := New(test.Context(), &stubSource{}, Config{})
	if err == nil {
		test.Fatal("accepted invalid constructor configuration")
	}
}

func assertInvalidSetting(test *testing.T, name string) {
	test.Helper()
	for _, value := range []string{"0", "-1", "invalid"} {
		assertSettingRejected(test, name, value)
	}
}

func assertSettingRejected(test *testing.T, name, value string) {
	test.Helper()
	_, err := ConfigFromEnv(func(key string) string { return map[string]string{name: value}[key] })
	if err == nil {
		test.Fatalf("accepted %s=%s", name, value)
	}
}
