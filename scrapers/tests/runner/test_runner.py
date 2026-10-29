import json
from dataclasses import replace

from inflacao_scrapers.adapters.registry import ADAPTERS
from inflacao_scrapers.delivery import Spool


def submissions(execution):
    return [
        json.loads(request.content) for request in execution.requests if request.method == "POST"
    ]


def test_registered_adapters_run_with_independent_settings_and_provenance(run_scrapers, tmp_path):
    execution = run_scrapers()
    documents = submissions(execution)
    assert execution.status == 0
    assert [document["shop_id"] for document in documents] == ["super-muffato", "fixture-shop"]
    assert [document["scraper_id"] for document in documents] == ["muffato-test", "fixture-test"]
    assert [document["collector"]["name"] for document in documents] == [
        "super-muffato",
        "fixture-shop",
    ]
    assert len({document["run_id"] for document in documents}) == 2
    assert [len(document["observations"]) for document in documents] == [2, 1]
    assert any(request.url.path == "/custom-catalog" for request in execution.requests)
    assert len(execution.clients) == 2
    assert all(client.is_closed for client in execution.clients)
    assert not (tmp_path / "data").exists()


def test_each_scraper_uses_its_own_core_and_token(run_scrapers):
    execution = run_scrapers()
    authenticated = [
        request for request in execution.requests if "authorization" in request.headers
    ]
    assert len(authenticated) == 4
    assert {request.url.host for request in authenticated} == {"core-a.example", "core-b.example"}
    assert all(
        request.headers["authorization"] == expected_token(request) for request in authenticated
    )


def expected_token(request):
    if request.url.host == "core-a.example":
        return "Bearer muffato-secret"
    return "Bearer fixture-secret"


def test_source_failure_does_not_prevent_later_scraper(run_scrapers, capsys):
    execution = run_scrapers(failures={"www.supermuffato.com.br": 403})
    assert execution.status == 1
    assert [document["shop_id"] for document in submissions(execution)] == ["fixture-shop"]
    assert all(client.is_closed for client in execution.clients)
    output = capsys.readouterr()
    assert "scrapers=2 failed=1 complete=False" in output.out
    assert "HTTP 403" in output.err


def test_core_scope_failure_does_not_prevent_later_scraper(run_scrapers):
    execution = run_scrapers(failures={"core-a.example": 401})
    assert execution.status == 1
    assert [document["shop_id"] for document in submissions(execution)] == ["fixture-shop"]
    assert not any(request.url.host == "www.supermuffato.com.br" for request in execution.requests)


def test_missing_token_does_not_prevent_later_scraper(run_scrapers, monkeypatch):
    monkeypatch.delenv("MUFFATO_TOKEN")
    execution = run_scrapers()
    assert execution.status == 1
    assert [document["shop_id"] for document in submissions(execution)] == ["fixture-shop"]
    assert all(client.is_closed for client in execution.clients)


def test_adapter_bug_does_not_prevent_later_scraper(run_scrapers, monkeypatch, capsys):
    def broken_pages(requester, settings, target, query):
        raise TypeError("Sensitive exception detail")

    broken_adapter = replace(ADAPTERS["super-muffato"], search_pages=broken_pages)
    monkeypatch.setitem(ADAPTERS, "super-muffato", broken_adapter)
    execution = run_scrapers()
    assert execution.status == 1
    assert [document["shop_id"] for document in submissions(execution)] == ["fixture-shop"]
    output = capsys.readouterr()
    assert "Unexpected TypeError" in output.err
    assert "Sensitive exception detail" not in output.err


def test_spool_write_failure_does_not_prevent_later_scraper(run_scrapers, monkeypatch, capsys):
    def unavailable_spool(self, identity, payload):
        raise OSError("Disk unavailable")

    monkeypatch.setattr(Spool, "enqueue_payload", unavailable_spool)
    execution = run_scrapers(failures={"super-muffato": 401})
    assert execution.status == 1
    assert len(submissions(execution)) == 2
    assert "Disk unavailable" in capsys.readouterr().err


def test_delivery_failure_saves_only_failed_scraper_and_continues(run_scrapers, tmp_path):
    execution = run_scrapers(failures={"super-muffato": 401})
    path = tmp_path / "data/super-muffato/muffato-test/spool.sqlite3"
    assert execution.status == 1
    assert len(submissions(execution)) == 2
    attempted = [request for request in execution.requests if request.method == "POST"]
    with Spool(path) as spool:
        assert spool.status()[0] == 1
        assert next(spool.pending())[1] == attempted[0].content
    assert not (tmp_path / "data/fixture-shop").exists()


def test_saved_batch_can_be_delivered_by_shared_runner(run_scrapers, tmp_path):
    failed = run_scrapers(failures={"super-muffato": 401})
    retried = run_scrapers(command="deliver")
    assert retried.status == 0
    assert submissions(failed)[0] == submissions(retried)[0]
    with Spool(tmp_path / "data/super-muffato/muffato-test/spool.sqlite3") as spool:
        assert spool.status() == (0, 0)


def test_status_reports_each_scrapers_queue(run_scrapers, capsys):
    run_scrapers(failures={"super-muffato": 401})
    capsys.readouterr()
    execution = run_scrapers(command="status")
    assert execution.status == 0
    assert not execution.requests
    output = capsys.readouterr().out
    assert "pending_batches=1" in output
    assert "pending_batches=0" in output


def test_discovery_uses_each_registered_adapter(run_scrapers, tmp_path):
    execution = run_scrapers(command="discover")
    assert execution.status == 0
    assert len(execution.requests) == 2
    assert not submissions(execution)
    assert not (tmp_path / "data").exists()
