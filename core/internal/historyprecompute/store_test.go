package historyprecompute

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"inflacao-real/core/internal/database"
	"inflacao-real/core/internal/insights"
	"inflacao-real/core/internal/testdatabase"
	"inflacao-real/core/internal/webapi"
)

type countingSource struct {
	webapi.PostgresStore
	calls       int
	failure     error
	failedGroup string
	during      func()
}

func (source *countingSource) History(requestContext context.Context, request webapi.HistoryRequest) (insights.History, error) {
	source.calls++
	if source.during != nil {
		source.during()
	}
	history := insights.Build(nil, request.Product == "global", 0, time.Now())
	history.Title = fmt.Sprintf("calculation %d", source.calls)
	history.Period = insights.Period{Start: request.Start, End: request.End}
	if source.failedGroup != "" && request.Group == source.failedGroup {
		return history, errors.New("group unavailable")
	}
	return history, source.failure
}

type fixture struct {
	test           *testing.T
	requestContext context.Context
	database       *sql.DB
	source         *countingSource
	store          *Store
}

func newFixture(test *testing.T) fixture {
	test.Helper()
	requestContext, pool := testdatabase.New(test, database.Open, database.Migrate)
	source := &countingSource{PostgresStore: webapi.PostgresStore{DB: pool}}
	return fixture{test, requestContext, pool, source, New(pool, source)}
}

func (fixture fixture) execute(query string, arguments ...any) {
	fixture.test.Helper()
	_, err := fixture.database.ExecContext(fixture.requestContext, query, arguments...)
	requireSuccess(fixture.test, err)
}

func requireSuccess(test *testing.T, err error) {
	test.Helper()
	if err != nil {
		test.Fatal(err)
	}
}

func requestForTest() webapi.HistoryRequest {
	return webapi.HistoryRequest{Product: "global", Start: "2026-10-01", End: "2026-11-01"}
}

func TestPersistedResultsSurviveRestartAndRejectOldRevision(test *testing.T) {
	fixture := newFixture(test)
	first, err := fixture.store.History(fixture.requestContext, requestForTest())
	requireSuccess(test, err)
	restarted := New(fixture.database, fixture.source)
	second, err := restarted.History(fixture.requestContext, requestForTest())
	requireSuccess(test, err)
	if first.Title != second.Title || fixture.source.calls != 1 {
		test.Fatal("restart recalculated saved history")
	}
	fixture.execute(`UPDATE catalog_decisions SET revision=revision+1`)
	third, err := restarted.History(fixture.requestContext, requestForTest())
	requireSuccess(test, err)
	if third.Title == first.Title || fixture.source.calls != 2 {
		test.Fatal("stale revision was served")
	}
}

func TestRevisionChangeDuringCalculationCannotPublishOldResult(test *testing.T) {
	fixture := newFixture(test)
	fixture.source.during = func() { fixture.execute(`UPDATE catalog_decisions SET revision=revision+1`) }
	_, err := fixture.store.History(fixture.requestContext, requestForTest())
	requireSuccess(test, err)
	var count int
	requireSuccess(test, fixture.database.QueryRowContext(fixture.requestContext, `SELECT count(*) FROM history_precomputations`).Scan(&count))
	if count != 0 {
		test.Fatal("revision race published a precomputation")
	}
}

func TestFailuresAreNotPersisted(test *testing.T) {
	fixture := newFixture(test)
	fixture.source.failure = errors.New("unavailable")
	_, err := fixture.store.History(fixture.requestContext, requestForTest())
	if err == nil {
		test.Fatal("calculation failure disappeared")
	}
	fixture.source.failure = nil
	_, err = fixture.store.History(fixture.requestContext, requestForTest())
	requireSuccess(test, err)
	if fixture.source.calls != 2 {
		test.Fatal("failure was cached")
	}
}

func TestRefreshWaitsForCommittedQueueAndRebuildsKnownSelections(test *testing.T) {
	fixture := newFixture(test)
	_, err := fixture.store.History(fixture.requestContext, requestForTest())
	requireSuccess(test, err)
	fixture.execute(`UPDATE catalog_decisions SET revision=revision+1`)
	fixture.execute(`INSERT INTO ingestion_payloads(scraper_id,submission_id,shop_id,schema_version,created_at,payload)
 VALUES('collector','00000000-0000-4000-8000-000000000001','shop','1.0',now(),'{}')`)
	requireSuccess(test, fixture.store.Refresh(fixture.requestContext, time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)))
	if fixture.source.calls != 1 {
		test.Fatal("refresh ran ahead of pending ingestion")
	}
	fixture.execute(`UPDATE ingestion_payloads SET status='processed',processed_at=now()`)
	requireSuccess(test, fixture.store.Refresh(fixture.requestContext, time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)))
	fixture.source.failure = errors.New("must use saved result")
	_, err = fixture.store.History(fixture.requestContext, requestForTest())
	requireSuccess(test, err)
	if fixture.source.calls != 6 {
		test.Fatalf("expected 5 default selections including the requested overview, calls=%d", fixture.source.calls)
	}
}

func TestBackgroundRefreshDoesNotExtendSelectionRetention(test *testing.T) {
	fixture := newFixture(test)
	request := requestForTest()
	_, err := fixture.store.History(fixture.requestContext, request)
	requireSuccess(test, err)
	identity, _ := selectionIdentity(request)
	fixture.execute(`UPDATE history_precomputations SET requested_at=now()-interval '8 days' WHERE identity=$1`, identity)
	_, err = fixture.store.calculate(fixture.requestContext, request, 0, false)
	requireSuccess(test, err)
	requireSuccess(test, fixture.store.prune(fixture.requestContext))
	_, err = fixture.store.lookup(fixture.requestContext, request, 0)
	if !errors.Is(err, sql.ErrNoRows) {
		test.Fatal("background refresh retained an unused selection")
	}
}

func TestDefaultOverviewMatchesWebMonthClamping(test *testing.T) {
	date := time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)
	if actual := previousMonth(date).Format(time.DateOnly); actual != "2026-02-28" {
		test.Fatalf("previous month=%s", actual)
	}
}

func TestMissedSignalWaitsForQuietIngestionBeforeRefresh(test *testing.T) {
	fixture := newFixture(test)
	fixture.execute(`UPDATE history_refresh_requests SET completed_generation=generation`)
	fixture.execute(`INSERT INTO ingestion_payloads(scraper_id,submission_id,shop_id,schema_version,created_at,payload,status,processed_at)
 VALUES('collector','00000000-0000-4000-8000-000000000001','shop','1.0',now(),'{}','processed',now())`)
	requireSuccess(test, fixture.store.Refresh(fixture.requestContext, time.Now()))
	if fixture.source.calls != 0 {
		test.Fatal("quiet fallback recalculated between incoming batches")
	}
	fixture.execute(`UPDATE ingestion_payloads SET processed_at=now()-interval '3 minutes'`)
	requireSuccess(test, fixture.store.Refresh(fixture.requestContext, time.Now()))
	if fixture.source.calls != 5 {
		test.Fatal("missed notification was not recovered after quiet interval")
	}
}

func TestFailedSelectionDoesNotBlockOtherGroups(test *testing.T) {
	fixture := newFixture(test)
	fixture.source.failedGroup = "doces"
	requireSuccess(test, fixture.store.Refresh(fixture.requestContext, time.Now()))
	var saved int
	requireSuccess(test, fixture.database.QueryRowContext(fixture.requestContext, `SELECT count(*) FROM history_precomputations WHERE result IS NOT NULL`).Scan(&saved))
	if saved != 4 || fixture.source.calls != 5 {
		test.Fatalf("saved=%d calls=%d", saved, fixture.source.calls)
	}
}
