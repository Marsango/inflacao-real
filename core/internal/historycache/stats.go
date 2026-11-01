package historycache

import "time"

// Bytes measures retained JSON payloads; Entries also bounds keys and bookkeeping.
// Durations measure underlying history calculations, excluding revision checks.
type Stats struct {
	Hits, Misses, Calculations, Failures, Evictions, Invalidations uint64
	Entries, Bytes                                                 int
	CalculationDuration                                            time.Duration
}

func (store *Store) Stats() Stats {
	cache := store.cache
	mutex := &cache.mutex
	mutex.Lock()
	defer mutex.Unlock()
	result := cache.statistics
	result.Entries = len(cache.entries)
	return result
}

func (cache *memory) recordRequest(hit bool) {
	mutex, statistics := &cache.mutex, &cache.statistics
	mutex.Lock()
	defer mutex.Unlock()
	if hit {
		statistics.Hits++
		return
	}
	statistics.Misses++
}

func (cache *memory) recordCalculation(duration time.Duration, err error) {
	mutex, statistics := &cache.mutex, &cache.statistics
	mutex.Lock()
	defer mutex.Unlock()
	statistics.Calculations++
	statistics.CalculationDuration += duration
	if err != nil {
		statistics.Failures++
	}
}
