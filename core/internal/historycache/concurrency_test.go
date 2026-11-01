package historycache

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"inflacao-real/core/internal/insights"
	"inflacao-real/core/internal/webapi"
)

type blockedFixture struct {
	source  *stubSource
	store   *Store
	started chan struct{}
	release chan struct{}
}

func newBlockedFixture(test *testing.T) blockedFixture {
	started, release := make(chan struct{}), make(chan struct{})
	source := &stubSource{history: func(requestContext context.Context, request webapi.HistoryRequest) (insights.History, error) {
		close(started)
		select {
		case <-release:
			return sampleHistory("shared"), nil
		case <-requestContext.Done():
			return insights.History{}, requestContext.Err()
		}
	}}
	return blockedFixture{source, newTestStore(test, source, DefaultConfig()), started, release}
}

func TestConcurrentMissesShareCalculation(test *testing.T) {
	fixture := newBlockedFixture(test)
	results := startReaders(test.Context(), fixture.store, 16)
	awaitSignal(test, fixture.started)
	close(fixture.release)
	for _, result := range results {
		assertHistoryTitle(test, receiveHistory(test, result), "shared")
	}
	assertCalls(test, fixture.source, 1)
}

func startReaders(requestContext context.Context, store *Store, count int) []<-chan historyResult {
	results := make([]<-chan historyResult, count)
	for position := range results {
		results[position] = startHistory(store, requestContext)
	}
	return results
}

func assertHistoryTitle(test *testing.T, result historyResult, title string) {
	test.Helper()
	requireSuccess(test, result.err)
	if result.history.Title != title {
		test.Fatalf("expected %q, got %q", title, result.history.Title)
	}
}

func TestCancelingInitiatorDoesNotCancelOtherReaders(test *testing.T) {
	fixture := newBlockedFixture(test)
	requestContext, cancel := context.WithCancel(test.Context())
	first := startHistory(fixture.store, requestContext)
	awaitSignal(test, fixture.started)
	second := startHistory(fixture.store, test.Context())
	cancel()
	if result := receiveHistory(test, first); !errors.Is(result.err, context.Canceled) {
		test.Fatalf("initiating reader did not cancel: %v", result.err)
	}
	close(fixture.release)
	assertHistoryTitle(test, receiveHistory(test, second), "shared")
	assertCalls(test, fixture.source, 1)
}

func TestCanceledFollowerReturnsWhileCalculationContinues(test *testing.T) {
	fixture := newBlockedFixture(test)
	first := startHistory(fixture.store, test.Context())
	awaitSignal(test, fixture.started)
	requestContext, cancel := context.WithCancel(test.Context())
	second := startHistory(fixture.store, requestContext)
	cancel()
	if result := receiveHistory(test, second); !errors.Is(result.err, context.Canceled) {
		test.Fatalf("follower did not cancel: %v", result.err)
	}
	close(fixture.release)
	assertHistoryTitle(test, receiveHistory(test, first), "shared")
	assertCalls(test, fixture.source, 1)
}

func TestApplicationShutdownCancelsSharedCalculation(test *testing.T) {
	fixture := newBlockedFixture(test)
	lifecycle, cancel := context.WithCancel(test.Context())
	fixture.store.lifecycle = lifecycle
	result := startHistory(fixture.store, test.Context())
	awaitSignal(test, fixture.started)
	cancel()
	if received := receiveHistory(test, result); !errors.Is(received.err, context.Canceled) {
		test.Fatalf("shared calculation survived shutdown: %v", received.err)
	}
}

func TestInvalidationDoesNotJoinOrRestoreOldCalculation(test *testing.T) {
	fixture := newBlockedFixture(test)
	var invocations atomic.Int64
	fixture.source.history = revisionHistory(fixture, &invocations)
	old := startHistory(fixture.store, test.Context())
	awaitSignal(test, fixture.started)
	fixture.source.revision.Store(1)
	current := startHistory(fixture.store, test.Context())
	assertHistoryTitle(test, receiveHistory(test, current), "new")
	close(fixture.release)
	assertHistoryTitle(test, receiveHistory(test, old), "old")
	if result := readHistory(test, fixture.store, defaultRequest()); result.Title != "new" {
		test.Fatal("old calculation replaced the current generation")
	}
	assertCalls(test, fixture.source, 2)
}

func revisionHistory(fixture blockedFixture, invocations *atomic.Int64) func(context.Context, webapi.HistoryRequest) (insights.History, error) {
	return func(requestContext context.Context, request webapi.HistoryRequest) (insights.History, error) {
		if invocations.Add(1) != 1 {
			return sampleHistory("new"), nil
		}
		close(fixture.started)
		select {
		case <-fixture.release:
			return sampleHistory("old"), nil
		case <-requestContext.Done():
			return insights.History{}, requestContext.Err()
		}
	}
}
