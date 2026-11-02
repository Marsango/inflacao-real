from types import SimpleNamespace

import httpx
import pytest
from stores.conftest import FIXTURES
from stores.test_pipeline import submissions

from inflacao_scrapers.delivery import Spool, deliver
from inflacao_scrapers.hirota import browser
from inflacao_scrapers.hirota.catalog import listing_pages
from inflacao_scrapers.hirota.config import API_BASE, CATALOG_PREFIX
from inflacao_scrapers.scope import MonitoringScope
from inflacao_scrapers.transport import Requester

from .pipeline import execute_hirota
from .test_parsing import prepared_settings


def test_collect_scope_delivers_hirota_with_provenance_without_spooling(tmp_path, monkeypatch):
    status, requests = execute_hirota(tmp_path, monkeypatch)
    assert status == 0
    document = submissions(requests)[0]
    assert document["shop_id"] == document["collector"]["name"] == "hirota"
    assert document["collector"]["version"]
    observations = document["observations"]
    assert len(observations) == 4
    assert all(item["context"]["channel"] == "pickup" for item in observations)
    assert all(item["source_url"].startswith(API_BASE) for item in observations)
    assert all(item["context"]["postal_code"] == "04263100" for item in observations)
    assert not list((tmp_path / "data").rglob("spool.sqlite3"))
    assert not any("/busca" in request.url.path for request in requests)


@pytest.mark.parametrize("failure", ["context", "catalog"])
def test_context_and_access_failures_prevent_delivery(tmp_path, monkeypatch, failure):
    status, requests = execute_hirota(tmp_path, monkeypatch, failure)
    assert status == 1 and not submissions(requests)
    assert not list((tmp_path / "data").rglob("spool.sqlite3"))
    if failure == "context":
        assert not any(request.url.path.endswith("/produtos") for request in requests)


def redeliver(path, attempted):
    requests = []
    with httpx.Client(
        transport=httpx.MockTransport(lambda request: accept(request, requests))
    ) as client:
        requester = Requester(client, interval=0)
        with Spool(path) as spool:
            assert next(spool.pending())[1] == attempted
            count = deliver(
                spool,
                requester,
                core_url="http://127.0.0.1:8080",
                token="secret-core-token",
                scraper_id="collector-sp-01",
                shop_id="hirota",
            )
            assert spool.status() == (0, 0)
    assert count == 1 and requests[0].content == attempted


def accept(request, requests):
    requests.append(request)
    return httpx.Response(200)


def test_delivery_failure_preserves_immutable_hirota_batch_for_retry(tmp_path, monkeypatch):
    status, requests = execute_hirota(tmp_path, monkeypatch, "delivery")
    assert status == 1
    attempted = next(request.content for request in requests if request.method == "POST")
    path = tmp_path / "data/hirota/collector-sp-01/spool.sqlite3"
    redeliver(path, attempted)


def test_catalog_cap_remains_incomplete_after_first_page():
    raw = (FIXTURES / "hirota-products.json").read_bytes()
    scope = MonitoringScope(schema_version="1.0", targets=[])
    with httpx.Client(
        transport=httpx.MockTransport(lambda _: httpx.Response(200, content=raw))
    ) as client:
        pages = listing_pages(
            Requester(client, interval=0),
            prepared_settings(),
            scope,
            CATALOG_PREFIX + "produtos/mais_vendidos",
            [1],
        )
        next(pages)
        with pytest.raises(ValueError, match="page limit"):
            next(pages)


def test_browser_captures_only_anonymous_catalog_session_headers():
    headers = {}
    request = SimpleNamespace(
        method="GET",
        url=API_BASE + CATALOG_PREFIX + "produtos/mais_vendidos?page=1",
        headers={"Authorization": "session", "X-Core-Token": "never-copy"},
    )
    browser.capture_headers(request, headers)
    assert headers == {"Authorization": "session"}
    headers.clear()
    request.url = "https://unexpected.example/" + request.url
    browser.capture_headers(request, headers)
    assert not headers
