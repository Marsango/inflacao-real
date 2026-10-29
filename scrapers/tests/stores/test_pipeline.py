import json
import sys

import httpx

from inflacao_scrapers import cli, commands
from inflacao_scrapers.delivery import Spool
from inflacao_scrapers.transport import Requester

CONFIGURATION = """
[[scrapers]]
adapter="angeloni"
scraper_id="shared"
catalog_category_ids=[467]
[[scrapers]]
adapter="festval"
scraper_id="shared"
"""


def monitoring_response(empty=False):
    targets = []
    if not empty:
        targets = [
            {
                "id": "pantry",
                "name": "Pantry",
                "search_queries": ["arroz", "chocolate"],
                "required_terms": [["chocolate", "arroz"]],
                "excluded_terms": [],
            }
        ]
    return httpx.Response(200, json={"schema_version": "1.0", "targets": targets})


def response_for(request, samples, failed_shop, empty, wrong_region):
    if request.url.host == "127.0.0.1":
        return core_response(request, failed_shop, empty)
    assert "authorization" not in request.headers
    name = "festval"
    if request.url.host == "super.angeloni.com.br":
        name = "angeloni"
    sample = samples[name]
    if request.url.path.endswith("/regions"):
        return httpx.Response(200, content=wrong_region.get(name, sample.regions))
    if "/category/tree/" in request.url.path:
        return httpx.Response(200, content=sample.categories)
    assert "vtex_segment=" in request.headers["cookie"]
    return httpx.Response(200, content=sample.source)


def core_response(request, failed_shop, empty):
    assert request.headers["authorization"] == "Bearer secret"
    if request.method == "POST":
        shop = json.loads(request.content)["shop_id"]
        return httpx.Response(401 if shop == failed_shop else 202)
    return monitoring_response(empty)


def execute(tmp_path, monkeypatch, samples, failed_shop=None, empty=False, wrong_region=None):
    requests = []

    def handler(request):
        requests.append(request)
        return response_for(request, samples, failed_shop, empty, wrong_region or {})

    path = tmp_path / "config.toml"
    path.write_text(CONFIGURATION)
    monkeypatch.setenv("INGESTION_TOKEN", "secret")
    install_mock_runtime(monkeypatch, handler)
    monkeypatch.setattr(sys, "argv", ["scraper", "--config", str(path), "collect"])
    return cli.main(), requests


def install_mock_runtime(monkeypatch, handler):
    monkeypatch.setattr(
        commands,
        "create_client",
        lambda settings: httpx.Client(transport=httpx.MockTransport(handler)),
    )
    monkeypatch.setattr(
        commands,
        "create_requester",
        lambda client, settings: Requester(client, interval=0, attempts=1),
    )


def submissions(requests):
    return [json.loads(request.content) for request in requests if request.method == "POST"]


def test_both_registered_scrapers_collect_and_deliver_without_local_storage(
    tmp_path, monkeypatch, samples, capsys
):
    status, requests = execute(tmp_path, monkeypatch, samples)
    documents = submissions(requests)
    assert status == 0
    assert [document["shop_id"] for document in documents] == ["angeloni", "festval"]
    assert [document["collector"]["name"] for document in documents] == ["angeloni", "festval"]
    assert [len(document["observations"]) for document in documents] == [1, 3]
    assert sum("product_search" in request.url.path for request in requests) == 1
    assert all(
        document["observations"][0]["context"]["postal_code"] == "80730201"
        for document in documents
    )
    assert not (tmp_path / "data").exists()
    assert "scrapers=2 failed=0 complete=True" in capsys.readouterr().out


def test_failed_angeloni_delivery_is_saved_and_festval_still_runs(tmp_path, monkeypatch, samples):
    status, requests = execute(tmp_path, monkeypatch, samples, failed_shop="angeloni")
    assert status == 1
    assert [document["shop_id"] for document in submissions(requests)] == ["angeloni", "festval"]
    with Spool(tmp_path / "data/angeloni/shared/spool.sqlite3") as spool:
        payload = next(spool.pending())[1]
        attempted = next(request.content for request in requests if request.method == "POST")
        assert payload == attempted
    assert not (tmp_path / "data/festval").exists()


def test_wrong_region_stops_that_scraper_before_prices_are_fetched(tmp_path, monkeypatch, samples):
    status, requests = execute(tmp_path, monkeypatch, samples, wrong_region={"angeloni": b"[]"})
    assert status == 1
    assert [document["shop_id"] for document in submissions(requests)] == ["festval"]
    assert not any("product_search" in request.url.path for request in requests)


def test_empty_core_scope_does_not_contact_either_shop(tmp_path, monkeypatch, samples):
    status, requests = execute(tmp_path, monkeypatch, samples, empty=True)
    assert status == 0
    assert len(requests) == 2
    assert all(request.url.host == "127.0.0.1" for request in requests)
    assert not (tmp_path / "data").exists()
