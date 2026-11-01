// Package historycache caches calculated public histories by committed catalog revision.
package historycache

import (
	"fmt"
	"strconv"
	"time"
)

type Config struct {
	TTL        time.Duration
	MaxEntries int
	MaxBytes   int
}

func DefaultConfig() Config {
	return Config{TTL: 5 * time.Minute, MaxEntries: 500, MaxBytes: 32 * 1024 * 1024}
}

func ConfigFromEnv(environment func(string) string) (Config, error) {
	configuration := DefaultConfig()
	if err := configuration.readTTL(environment("HISTORY_CACHE_TTL")); err != nil {
		return configuration, err
	}
	if err := readLimit(environment, "HISTORY_CACHE_MAX_ENTRIES", &configuration.MaxEntries); err != nil {
		return configuration, err
	}
	err := readLimit(environment, "HISTORY_CACHE_MAX_BYTES", &configuration.MaxBytes)
	return configuration, err
}

func (configuration *Config) readTTL(value string) error {
	if value == "" {
		return nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return fmt.Errorf("HISTORY_CACHE_TTL must be a positive duration")
	}
	configuration.TTL = duration
	return nil
}

func readLimit(environment func(string) string, name string, target *int) error {
	value := environment(name)
	if value == "" {
		return nil
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit <= 0 {
		return fmt.Errorf("%s must be a positive integer", name)
	}
	*target = limit
	return nil
}
