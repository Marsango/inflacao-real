import json
import sys

import httpx
from stores.test_pipeline import install_mock_runtime, monitoring_response

from inflacao_scrapers import cli
from inflacao_scrapers.hirota import session

from .test_parsing import source


def execute_hirota(tmp_path, monkeypatch, failure=None):
    requests = []
    path = tmp_path / "config.toml"
    path.write_text('[[scrapers]]\nadapter="hirota"\nscraper_id="collector-sp-01"\n')
    monkeypatch.setenv("INGESTION_TOKEN", "secret-core-token")
    monkeypatch.setattr(session, "bootstrap_session", lambda _: {"Authorization": "anonymous"})
    install_mock_runtime(monkeypatch, handler(requests, failure))
    monkeypatch.setattr(sys, "argv", ["scraper", "--config", str(path), "collect"])
    return cli.main(), requests


def handler(requests, failure):
    def respond(request):
        if request.url.path == "/api/v1/ingestion/scraper-reports":
            return httpx.Response(201)
        requests.append(request)
        if request.url.host == "127.0.0.1":
            return core_response(request, failure)
        assert request.headers["authorization"] == "anonymous"
        return shop_response(request, failure)

    return respond


def core_response(request, failure):
    assert request.headers["authorization"] == "Bearer secret-core-token"
    if request.method == "GET":
        return monitoring_response()
    return httpx.Response(401 if failure == "delivery" else 202)


def shop_response(request, failure):
    if request.url.path.endswith("/centros_distribuicoes/1"):
        return context_response(failure)
    if request.url.path.endswith("/centros_distribuicoes/retiradas"):
        return httpx.Response(200, json={"success": True, "data": [source("context")["data"]]})
    if request.url.path.endswith("/arvore"):
        return httpx.Response(
            200, json={"success": True, "data": [{"classificacao_mercadologica_id": 6}]}
        )
    return catalog_response(failure)


def context_response(failure):
    branch = source("context")
    if failure == "context":
        branch["data"]["endereco"]["cidade"] = "Guarulhos"
    return httpx.Response(200, json=branch)


def catalog_response(failure):
    if failure == "catalog":
        return httpx.Response(403)
    document = source("cereals")
    document["paginator"].update(total_pages=1, total_items=20)
    return httpx.Response(200, content=json.dumps(document, default=str).encode())
