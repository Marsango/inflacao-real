package historycache

import (
	"container/list"
	"sync"
	"time"

	"inflacao-real/core/internal/webapi"
)

type cacheKey struct {
	Request     webapi.HistoryRequest
	Revision    int64
	Methodology string
}

type entry struct {
	key       cacheKey
	data      []byte
	expiresAt time.Time
}

type memory struct {
	mutex         sync.Mutex
	configuration Config
	entries       map[cacheKey]*list.Element
	recent        list.List
	revision      int64
	statistics    Stats
}

func newMemory(configuration Config) *memory {
	return &memory{configuration: configuration, entries: make(map[cacheKey]*list.Element)}
}

func (cache *memory) lookup(key cacheKey, now time.Time) ([]byte, bool) {
	mutex, recent := &cache.mutex, &cache.recent
	mutex.Lock()
	defer mutex.Unlock()
	cache.advance(key.Revision)
	element, found := cache.entries[key]
	if !found {
		return nil, false
	}
	value := element.Value.(entry)
	if !now.Before(value.expiresAt) {
		cache.remove(element)
		return nil, false
	}
	recent.MoveToFront(element)
	return value.data, true
}

// advance runs under mutex. Older in-flight requests cannot restore an old generation.
func (cache *memory) advance(revision int64) {
	if revision <= cache.revision {
		return
	}
	cache.entries = make(map[cacheKey]*list.Element)
	recent, statistics := &cache.recent, &cache.statistics
	recent.Init()
	statistics.Bytes = 0
	statistics.Invalidations++
	cache.revision = revision
}

func (cache *memory) put(key cacheKey, data []byte, now time.Time) {
	mutex, recent, statistics := &cache.mutex, &cache.recent, &cache.statistics
	mutex.Lock()
	defer mutex.Unlock()
	configuration := cache.configuration
	if key.Revision != cache.revision || len(data) > configuration.MaxBytes {
		return
	}
	if previous, found := cache.entries[key]; found {
		cache.remove(previous)
	}
	cache.makeRoom(len(data))
	value := entry{key: key, data: data, expiresAt: now.Add(configuration.TTL)}
	cache.entries[key] = recent.PushFront(value)
	statistics.Bytes += len(data)
}

func (cache *memory) makeRoom(size int) {
	configuration := cache.configuration
	recent, statistics := &cache.recent, &cache.statistics
	for len(cache.entries) >= configuration.MaxEntries || size > configuration.MaxBytes-statistics.Bytes {
		cache.remove(recent.Back())
		statistics.Evictions++
	}
}

func (cache *memory) remove(element *list.Element) {
	value := element.Value.(entry)
	delete(cache.entries, value.key)
	recent, statistics := &cache.recent, &cache.statistics
	statistics.Bytes -= len(value.data)
	recent.Remove(element)
}
