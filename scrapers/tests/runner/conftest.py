import json
import sys
from dataclasses import dataclass
from pathlib import Path
from uuid import uuid4

import httpx
import pytest

from inflacao_scrapers import cli, commands
from inflacao_scrapers.adapters import Adapter, ParsedPage
from inflacao_scrapers.adapters.registry import ADAPTERS
from inflacao_scrapers.config import Settings
from inflacao_scrapers.models import Observation
from inflacao_scrapers.transport import Requester

MUFFATO_CONFIGURATION = """
[[scrapers]]
adapter = "super-muffato"
scraper_id = "muffato-test"
core_url = "https://core-a.example"
ingestion_token_env = "MUFFATO_TOKEN"
sales_channel = "8"
"""

ADDITIONAL_CONFIGURATION = """
[[scrapers]]
adapter = "fixture-shop"
scraper_id = "fixture-test"
shop_id = "fixture-shop"
shop_url = "https://fixture-shop.example"
core_url = "https://core-b.example"
ingestion_token_env = "FIXTURE_TOKEN"
catalog_path = "/custom-catalog"
"""


class FixtureSettings(Settings):
    catalog_path: str


def fixture_page(requester, settings):
    response = requester.request("GET", settings.shop_url + settings.catalog_path)
    if response.status_code != 200:
        raise ValueError(f"Fixture source HTTP {response.status_code}")
    return ParsedPage([Observation.model_validate_json(response.content)])


def fixture_pages(requester, settings, target, query):
    yield fixture_page(requester, settings)


def fixture_discover(requester, settings, query, limit):
    return fixture_page(requester, settings)


@pytest.fixture
def additional_adapter(monkeypatch):
    adapter = Adapter("fixture-shop", FixtureSettings, fixture_pages, fixture_discover)
    monkeypatch.setitem(ADAPTERS, adapter.name, adapter)
    return adapter


def monitoring_response(status=200):
    if status != 200:
        return httpx.Response(status)
    target = dict(
        id="rice",
        name="Rice",
        search_queries=["arroz"],
        required_terms=[["arroz"]],
        excluded_terms=[],
    )
    return httpx.Response(200, json={"schema_version": "1.0", "targets": [target]})


def response_for(request, source_bytes, observation, failures):
    host = request.url.host
    if request.url.path == "/api/v1/monitoring-targets":
        return monitoring_response(failures.get(host, 200))
    if request.method == "POST":
        document = json.loads(request.content)
        return httpx.Response(failures.get(document["shop_id"], 202))
    assert "authorization" not in request.headers
    if host in failures:
        return httpx.Response(failures[host])
    if host == "www.supermuffato.com.br":
        return httpx.Response(206, content=source_bytes)
    document = observation.model_dump(mode="json")
    document["observation_id"] = str(uuid4())
    return httpx.Response(200, json=document)


@dataclass
class Execution:
    status: int
    requests: list
    clients: list
    configuration_path: Path


@pytest.fixture
def run_scrapers(tmp_path, monkeypatch, source_bytes, observation, additional_adapter):
    monkeypatch.setenv("INGESTION_TOKEN", "default-secret")
    monkeypatch.setenv("MUFFATO_TOKEN", "muffato-secret")
    monkeypatch.setenv("FIXTURE_TOKEN", "fixture-secret")

    def execute(configuration=None, command="collect", failures=None):
        path = tmp_path / "config.toml"
        path.write_text(configuration or MUFFATO_CONFIGURATION + ADDITIONAL_CONFIGURATION)
        requests, clients = [], []
        handler = recording_handler(requests, source_bytes, observation, failures or {})
        install_clients(monkeypatch, handler, clients)
        arguments = ["scraper", "--config", str(path), command]
        if command == "discover":
            arguments.extend(["--query", "arroz", "--limit", "10"])
        monkeypatch.setattr(sys, "argv", arguments)
        return Execution(cli.main(), requests, clients, path)

    return execute


def recording_handler(requests, source_bytes, observation, failures):
    def handler(request):
        requests.append(request)
        return response_for(request, source_bytes, observation, failures)

    return handler


def install_clients(monkeypatch, handler, clients):
    def create_client(settings):
        client = httpx.Client(transport=httpx.MockTransport(handler))
        clients.append(client)
        return client

    monkeypatch.setattr(commands, "create_client", create_client)
    monkeypatch.setattr(
        commands,
        "create_requester",
        lambda client, settings: Requester(client, interval=0, attempts=1),
    )
