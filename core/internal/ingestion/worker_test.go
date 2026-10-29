package ingestion

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWorkerConfigurationDefaults(test *testing.T) {
	configuration, err := WorkerConfigFromEnv(func(string) string { return "" })
	if err != nil || configuration != DefaultWorkerConfig() {
		test.Fatalf("defaults: %+v, %v", configuration, err)
	}
}

func configuredWorker(test *testing.T) WorkerConfig {
	values := map[string]string{"INGESTION_POLL_INTERVAL": "2s", "INGESTION_BATCH_SIZE": "3",
		"INGESTION_MAX_ATTEMPTS": "4", "INGESTION_RETRY_BASE": "10ms", "INGESTION_RETRY_MAX": "35ms"}
	configuration, err := WorkerConfigFromEnv(func(key string) string { return values[key] })
	requireNoError(test, err)
	return configuration
}

func TestWorkerConfigurationOverrides(test *testing.T) {
	configuration := configuredWorker(test)
	if configuration.BatchSize != 3 || configuration.PollInterval != 2*time.Second || configuration.MaxAttempts != 4 {
		test.Fatalf("overrides: %+v", configuration)
	}
}

func TestWorkerBackoff(test *testing.T) {
	configuration := configuredWorker(test)
	for position, expected := range []time.Duration{10, 20, 35, 35, 35} {
		assertRetryDelay(test, configuration, position+1, expected*time.Millisecond)
	}
	configuration.RetryBase, configuration.RetryMax = time.Duration(1<<62), time.Duration(1<<63-1)
	assertRetryDelay(test, configuration, 100, configuration.RetryMax)
}

func assertRetryDelay(test *testing.T, configuration WorkerConfig, attempt int, expected time.Duration) {
	if actual := configuration.retryDelay(attempt); actual != expected {
		test.Fatalf("attempt %d delay=%v want=%v", attempt, actual, expected)
	}
}

func TestWorkerRejectsInvalidConfiguration(test *testing.T) {
	values := map[string]string{"INGESTION_POLL_INTERVAL": "0s", "INGESTION_BATCH_SIZE": "-1",
		"INGESTION_MAX_ATTEMPTS": "x", "INGESTION_RETRY_BASE": "-1s", "INGESTION_RETRY_MAX": "1s"}
	for key, value := range values {
		test.Run(key, func(test *testing.T) { rejectWorkerConfiguration(test, key, value) })
	}
}

func rejectWorkerConfiguration(test *testing.T, key, value string) {
	values := map[string]string{key: value}
	_, err := WorkerConfigFromEnv(func(name string) string { return values[name] })
	if err == nil {
		test.Fatal("invalid setting accepted")
	}
}

func TestWorkerDrainsAcrossBatchesBeforeIdling(test *testing.T) {
	requestContext, cancel := context.WithCancel(test.Context())
	defer cancel()
	calls, idleCalls := 0, 0
	runLoop(requestContext, 2, func(context.Context) (bool, error) {
		calls++
		return calls <= 5, nil
	}, func(context.Context) {
		idleCalls++
		if calls != 6 {
			test.Fatalf("idled with ready work: calls=%d", calls)
		}
		cancel()
	})
	if calls != 6 || idleCalls != 1 {
		test.Fatalf("calls=%d idle=%d", calls, idleCalls)
	}
}

func TestWorkerWaitsAfterInfrastructureErrorAndResumes(test *testing.T) {
	requestContext, cancel := context.WithCancel(test.Context())
	defer cancel()
	calls, waits := 0, 0
	runLoop(requestContext, 2, func(context.Context) (bool, error) {
		calls++
		if calls == 1 {
			return false, errors.New("database unavailable")
		}
		cancel()
		return true, nil
	}, func(context.Context) { waits++ })
	if calls != 2 || waits != 1 {
		test.Fatalf("calls=%d waits=%d", calls, waits)
	}
}

func TestWorkerCancellation(test *testing.T) {
	requestContext, cancel := context.WithCancel(test.Context())
	cancel()
	runLoop(requestContext, 2, func(context.Context) (bool, error) {
		test.Fatal("canceled worker started processing")
		return false, nil
	}, func(context.Context) { test.Fatal("canceled worker waited") })

	requestContext, cancel = context.WithCancel(test.Context())
	defer cancel()
	calls := 0
	runLoop(requestContext, 2, func(context.Context) (bool, error) {
		calls++
		cancel()
		return true, nil
	}, func(context.Context) { test.Fatal("worker waited after cancellation") })
	if calls != 1 {
		test.Fatalf("worker continued after cancellation: %d", calls)
	}
}
