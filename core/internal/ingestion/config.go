package ingestion

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

type WorkerConfig struct {
	PollInterval time.Duration
	BatchSize    int
	MaxAttempts  int
	RetryBase    time.Duration
	RetryMax     time.Duration
}

func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{PollInterval: time.Minute, BatchSize: 100, MaxAttempts: 5, RetryBase: time.Minute, RetryMax: time.Hour}
}

func WorkerConfigFromEnv(environment func(string) string) (WorkerConfig, error) {
	configuration := DefaultWorkerConfig()
	if err := configuration.loadDurations(environment); err != nil {
		return configuration, err
	}
	if err := configuration.loadCounts(environment); err != nil {
		return configuration, err
	}
	return configuration, configuration.validate()
}

func (configuration *WorkerConfig) loadDurations(environment func(string) string) error {
	targets := map[string]*time.Duration{"INGESTION_POLL_INTERVAL": &configuration.PollInterval,
		"INGESTION_RETRY_BASE": &configuration.RetryBase, "INGESTION_RETRY_MAX": &configuration.RetryMax}
	var err error
	for name, target := range targets {
		err = readDuration(err, name, environment(name), target)
	}
	return err
}

func readDuration(previous error, name, value string, target *time.Duration) error {
	if previous != nil || value == "" {
		return previous
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return fmt.Errorf("%s must be a positive duration", name)
	}
	*target = duration
	return nil
}

func (configuration *WorkerConfig) loadCounts(environment func(string) string) error {
	targets := map[string]*int{"INGESTION_BATCH_SIZE": &configuration.BatchSize, "INGESTION_MAX_ATTEMPTS": &configuration.MaxAttempts}
	var err error
	for name, target := range targets {
		err = readCount(err, name, environment(name), target)
	}
	return err
}

func readCount(previous error, name, value string, target *int) error {
	if previous != nil || value == "" {
		return previous
	}
	count, err := strconv.Atoi(value)
	if err != nil || count <= 0 {
		return fmt.Errorf("%s must be a positive integer", name)
	}
	*target = count
	return nil
}

func (configuration WorkerConfig) validate() error {
	if configuration.PollInterval <= 0 || configuration.BatchSize <= 0 || configuration.MaxAttempts <= 0 || configuration.RetryBase <= 0 || configuration.RetryMax < configuration.RetryBase {
		return errors.New("worker settings must be positive and retry max must be at least retry base")
	}
	return nil
}

func (configuration WorkerConfig) retryDelay(attempt int) time.Duration {
	delay := configuration.RetryBase
	for position := 1; position < attempt && delay < configuration.RetryMax; position++ {
		delay = configuration.doubleDelay(delay)
	}
	return delay
}

func (configuration WorkerConfig) doubleDelay(delay time.Duration) time.Duration {
	if delay >= configuration.RetryMax-delay {
		return configuration.RetryMax
	}
	return delay * 2
}
