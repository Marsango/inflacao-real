package ingestion

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWorkerConfigurationAndBackoff(t *testing.T) {
	c, err := WorkerConfigFromEnv(func(string) string { return "" })
	if err != nil || c != DefaultWorkerConfig() {
		t.Fatalf("defaults: %+v, %v", c, err)
	}
	values := map[string]string{"INGESTION_POLL_INTERVAL": "2s", "INGESTION_BATCH_SIZE": "3",
		"INGESTION_MAX_ATTEMPTS": "4", "INGESTION_RETRY_BASE": "10ms", "INGESTION_RETRY_MAX": "35ms"}
	c, err = WorkerConfigFromEnv(func(key string) string { return values[key] })
	if err != nil || c.BatchSize != 3 || c.PollInterval != 2*time.Second || c.MaxAttempts != 4 {
		t.Fatalf("overrides: %+v, %v", c, err)
	}
	for i, want := range []time.Duration{10, 20, 35, 35, 35} {
		if got := c.retryDelay(i + 1); got != want*time.Millisecond {
			t.Fatalf("attempt %d delay=%v want=%v", i+1, got, want*time.Millisecond)
		}
	}
	// Doubling near the duration limit must cap without overflowing.
	c.RetryBase, c.RetryMax = time.Duration(1<<62), time.Duration(1<<63-1)
	if got := c.retryDelay(100); got != c.RetryMax {
		t.Fatalf("backoff overflow: %v", got)
	}
	for key, value := range map[string]string{
		"INGESTION_POLL_INTERVAL": "0s", "INGESTION_BATCH_SIZE": "-1",
		"INGESTION_MAX_ATTEMPTS": "x", "INGESTION_RETRY_BASE": "-1s", "INGESTION_RETRY_MAX": "1s",
	} {
		t.Run(key, func(t *testing.T) {
			_, err := WorkerConfigFromEnv(func(name string) string {
				if name == key {
					return value
				}
				return ""
			})
			if err == nil {
				t.Fatal("invalid setting accepted")
			}
		})
	}
}

func TestWorkerDrainsAcrossBatchesBeforeIdling(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	calls, idleCalls := 0, 0
	runLoop(ctx, 2, func(context.Context) (bool, error) {
		calls++
		return calls <= 5, nil
	}, func(context.Context) {
		idleCalls++
		if calls != 6 {
			t.Fatalf("idled with ready work: calls=%d", calls)
		}
		cancel()
	})
	if calls != 6 || idleCalls != 1 {
		t.Fatalf("calls=%d idle=%d", calls, idleCalls)
	}
}

func TestWorkerWaitsAfterInfrastructureErrorAndResumes(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	calls, waits := 0, 0
	runLoop(ctx, 2, func(context.Context) (bool, error) {
		calls++
		if calls == 1 {
			return false, errors.New("database unavailable")
		}
		cancel()
		return true, nil
	}, func(context.Context) { waits++ })
	if calls != 2 || waits != 1 {
		t.Fatalf("calls=%d waits=%d", calls, waits)
	}
}

func TestWorkerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	runLoop(ctx, 2, func(context.Context) (bool, error) {
		t.Fatal("canceled worker started processing")
		return false, nil
	}, func(context.Context) { t.Fatal("canceled worker waited") })

	ctx, cancel = context.WithCancel(t.Context())
	defer cancel()
	calls := 0
	runLoop(ctx, 2, func(context.Context) (bool, error) {
		calls++
		cancel()
		return true, nil
	}, func(context.Context) { t.Fatal("worker waited after cancellation") })
	if calls != 1 {
		t.Fatalf("worker continued after cancellation: %d", calls)
	}
}
