import json

import httpx
import pytest
from stores.test_pipeline import submissions

from inflacao_scrapers.delivery import Spool, deliver
from inflacao_scrapers.transport import Requester

from .pipeline import execute_nagumo
from .pipeline import grid_response as original_grid_response


def test_cli_collects_core_scope_with_pickup_provenance_without_creating_spool(
    tmp_path, monkeypatch, capsys
):
    status, requests = execute_nagumo(tmp_path, monkeypatch)
    assert status == 0
    documents = submissions(requests)
    assert documents and all(document["shop_id"] == "nagumo" for document in documents)
    item = documents[0]["observations"][0]
    assert item["context"]["store_id"] == "13" and item["context"]["postal_code"] == "08030040"
    assert item["context"]["channel"] == "pickup" and item["context"]["city"] == "São Paulo"
    assert item["source_url"].startswith("https://www.nagumo.com.br/on/demandware.store/")
    assert documents[0]["collector"]["name"] == "nagumo" and documents[0]["collector"]["version"]
    assert not (tmp_path / "data").exists()
    assert "matched_listings=" in capsys.readouterr().out


@pytest.mark.parametrize(
    "failure", ["context", "selection", "catalog", "lost-session", "lost-during-grid"]
)
def test_context_and_access_failures_prevent_delivery(tmp_path, monkeypatch, failure):
    status, requests = execute_nagumo(tmp_path, monkeypatch, failure)
    assert status == 1 and not submissions(requests)
    assert not (tmp_path / "data").exists()


def test_search_page_cap_never_reports_complete(tmp_path, monkeypatch, capsys):
    status, requests = execute_nagumo(tmp_path, monkeypatch, "pagination-cap")
    assert status == 1 and submissions(requests)
    assert "page limit" in capsys.readouterr().err


def test_failed_delivery_preserves_the_exact_attempt_for_safe_retry(tmp_path, monkeypatch):
    status, requests = execute_nagumo(tmp_path, monkeypatch, "delivery")
    assert status == 1
    attempted = next(request.content for request in requests if request.method == "POST")
    path = tmp_path / "data/nagumo/collector-sp-01/spool.sqlite3"
    with Spool(path) as spool:
        assert next(spool.pending())[1] == attempted
        redeliver(spool, attempted)
        assert spool.status() == (0, 0)


def redeliver(spool, attempted):
    received = []
    with httpx.Client(
        transport=httpx.MockTransport(lambda request: accept(request, received))
    ) as client:
        count = deliver(
            spool,
            Requester(client, interval=0),
            core_url="http://127.0.0.1:8080",
            token="secret-core-token",
            scraper_id="collector-sp-01",
            shop_id="nagumo",
        )
    assert count == 1 and received[0].content == attempted
    assert json.loads(attempted)["shop_id"] == "nagumo"


def accept(request, received):
    received.append(request)
    return httpx.Response(200)


def unavailable_grid(request, failure):
    response = original_grid_response(request, failure)
    document = json.loads(response.content)
    mark_unavailable(document["productsSearchResult"])
    return httpx.Response(200, content=json.dumps(document))


def mark_unavailable(products):
    for product in products:
        product["available"] = False


def test_discovery_coverage_is_distinct_from_available_price_counts(
    tmp_path, monkeypatch, capsys
):
    from . import pipeline

    monkeypatch.setattr(pipeline, "grid_response", unavailable_grid)
    status, requests = execute_nagumo(tmp_path, monkeypatch)
    observations = [item for batch in submissions(requests) for item in batch["observations"]]
    assert status == 0 and observations
    assert all(item["offers"] == [] for item in observations)
    output = capsys.readouterr().out
    assert "matched_listings=" in output and "available_price_observations=0" in output
    assert f"unavailable_observations={len(observations)}" in output
