import json
import sys

import httpx
from stores.test_pipeline import install_mock_runtime, monitoring_response

from inflacao_scrapers import cli

from .conftest import source


def execute_nagumo(tmp_path, monkeypatch, failure=None):
    requests = []
    path = tmp_path / "config.toml"
    path.write_text(
        '[[scrapers]]\nadapter="nagumo"\nscraper_id="collector-sp-01"\nmax_search_pages=1\n'
    )
    monkeypatch.setenv("INGESTION_TOKEN", "secret-core-token")
    install_mock_runtime(monkeypatch, handler(requests, failure))
    monkeypatch.setattr(sys, "argv", ["scraper", "--config", str(path), "collect"])
    return cli.main(), requests


def handler(requests, failure):
    def respond(request):
        requests.append(request)
        if request.url.host == "127.0.0.1":
            return core_response(request, failure)
        assert "authorization" not in request.headers
        return shop_response(request, current_failure(requests, failure))

    return respond


def current_failure(requests, failure):
    if failure != "lost-during-grid":
        return failure
    if any(request.url.path.endswith("/Search-UpdateGrid") for request in requests):
        return "lost-session"
    return None


def core_response(request, failure):
    assert request.headers["authorization"] == "Bearer secret-core-token"
    if request.method == "GET":
        return monitoring_response()
    return httpx.Response(401 if failure == "delivery" else 202)


def shop_response(request, failure):
    route = request.url.path.rsplit("/", 1)[1]
    if route == "Cart-Get":
        return document_response({"action": route, "numItems": 0, "items": []})
    if route == "Stores-SelectStore":
        assert dict(request.url.params) == {"storeId": "13", "method": "Retirada"}
        return selection_response(failure)
    if route == "Stores-AvailableStores":
        return context_response(request, failure)
    assert "selected=13" in request.headers.get("cookie", "")
    if failure == "catalog":
        return httpx.Response(403)
    return grid_response(request, failure)


def selection_response(failure):
    document = source("selection")
    document["success"] = failure != "selection"
    return document_response(document, {"Set-Cookie": "selected=13; Path=/; Secure"})


def context_response(request, failure):
    document = source("context")
    active = next(store for store in document["stores"] if store["active"])
    if failure == "context":
        active["city"] = "GUARULHOS"
    if failure == "lost-session" and "selected=13" in request.headers.get("cookie", ""):
        active["active"] = False
    return document_response(document)


def grid_response(request, failure):
    document = source("rice")
    if failure != "pagination-cap":
        document["count"] = document["productSearch"]["count"] = 24
        document["productSearch"]["showMoreUrl"] = "finished"
    document["productSearch"]["searchKeywords"] = request.url.params["q"]
    return document_response(document)


def document_response(document, headers=None):
    return httpx.Response(200, content=json.dumps(document, default=str).encode(), headers=headers)
