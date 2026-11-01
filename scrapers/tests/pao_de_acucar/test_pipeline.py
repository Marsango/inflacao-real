import json
import sys

import httpx
import pytest
from stores.test_pipeline import install_mock_runtime, monitoring_response

from inflacao_scrapers import cli
from inflacao_scrapers.delivery import Spool, deliver
from inflacao_scrapers.pao_de_acucar.catalog import category_pages
from inflacao_scrapers.pao_de_acucar.config import PaoDeAcucarSettings
from inflacao_scrapers.scope import MonitoringScope
from inflacao_scrapers.transport import Requester

from .test_source import FIXTURES, settings


def shop_response(request, failure):
    if request.url.path.endswith("deliveryOptions"):
        return context_response(failure)
    if request.url.path.endswith("humanization"):
        return fixture_response("branch")
    if request.url.path.endswith("categories"):
        return httpx.Response(200, json={"content": [{"uiLink": "/alimentos"}]})
    return single_page(request)


def context_response(failure):
    if failure == "context":
        return httpx.Response(200, json={"deliveryTypes": []})
    return fixture_response("delivery")


def fixture_response(name):
    return httpx.Response(200, content=(FIXTURES / f"pao-de-acucar-{name}.json").read_bytes())


def single_page(request):
    raw = (FIXTURES / "pao-de-acucar-rice.json").read_bytes()
    raw = raw.replace(b'"totalProducts":58', b'"totalProducts":12')
    raw = raw.replace(b'"totalPages":5', b'"totalPages":1')
    assert json.loads(request.content)["storeId"] == 461
    return httpx.Response(200, content=raw)


def execute(tmp_path, monkeypatch, failure=None):
    requests = []

    def respond(request):
        requests.append(request)
        if request.url.host != "127.0.0.1":
            assert "authorization" not in request.headers
            return shop_response(request, failure)
        if request.method == "GET":
            return monitoring_response()
        return httpx.Response(401 if failure == "delivery" else 202)

    path = tmp_path / "config.toml"
    path.write_text('[[scrapers]]\nadapter="pao-de-acucar"\nscraper_id="collector-sp-01"\n')
    monkeypatch.setenv("INGESTION_TOKEN", "secret")
    install_mock_runtime(monkeypatch, respond)
    monkeypatch.setattr(sys, "argv", ["scraper", "--config", str(path), "collect"])
    return cli.main(), requests


def submission_requests(requests):
    return [request for request in requests if request.url.path.endswith("submissions")]


def test_real_runner_delivers_with_provenance(tmp_path, monkeypatch):
    status, requests = execute(tmp_path, monkeypatch)
    assert status == 0
    document = json.loads(submission_requests(requests)[0].content)
    assert document["shop_id"] == document["collector"]["name"] == "pao-de-acucar"
    assert len(document["observations"]) == 12
    assert document["observations"][0]["context"]["postal_code"] == "04115090"
    assert not (tmp_path / "data").exists()
    assert not any("/busca" in str(request.url) for request in requests)


def test_missing_context_prevents_prices_and_delivery(tmp_path, monkeypatch):
    status, requests = execute(tmp_path, monkeypatch, "context")
    assert status == 1 and not submission_requests(requests)
    assert not any(request.url.path.endswith("category-page") for request in requests)


def test_failed_delivery_spools_identical_attempted_bytes(tmp_path, monkeypatch):
    status, requests = execute(tmp_path, monkeypatch, "delivery")
    assert status == 1
    attempted = submission_requests(requests)[0].content
    with Spool(tmp_path / "data/pao-de-acucar/collector-sp-01/spool.sqlite3") as spool:
        assert next(spool.pending())[1] == attempted


def test_page_budget_failure_remains_incomplete():
    raw = (FIXTURES / "pao-de-acucar-rice.json").read_bytes()
    with httpx.Client(
        transport=httpx.MockTransport(lambda _: httpx.Response(200, content=raw))
    ) as client:
        pages = category_pages(
            Requester(client, interval=0),
            settings(),
            MonitoringScope(schema_version="1.0", targets=[]),
            "alimentos",
            [1],
        )
        next(pages)
        with pytest.raises(ValueError, match="page limit"):
            next(pages)


def test_unresolved_settings_cannot_collect():
    with pytest.raises(ValueError, match="resolved"):
        PaoDeAcucarSettings(scraper_id="diagnostic").require_context()



def test_spooled_retry_reuses_original_bytes(tmp_path, monkeypatch):
    _, requests = execute(tmp_path, monkeypatch, "delivery")
    attempted = submission_requests(requests)[0].content
    retried = []
    def response(request):
        return record_retry(request, retried)
    with httpx.Client(transport=httpx.MockTransport(response)) as client:
        with Spool(tmp_path / "data/pao-de-acucar/collector-sp-01/spool.sqlite3") as spool:
            count = deliver(spool, Requester(client, interval=0), core_url="http://127.0.0.1:8080",
                            token="secret", scraper_id="collector-sp-01", shop_id="pao-de-acucar")
            assert spool.status() == (0, 0)
    assert count == 1 and retried == [attempted]


def record_retry(request, retried):
    retried.append(request.content)
    return httpx.Response(200)
