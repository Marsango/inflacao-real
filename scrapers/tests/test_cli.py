import json
import sys
from pathlib import Path

import httpx

from inflacao_scrapers import cli, commands
from inflacao_scrapers.config import load_settings
from inflacao_scrapers.delivery import Spool
from inflacao_scrapers.transport import Requester


def target(identity="rice", query="arroz"):
    return {
        "id": identity,
        "name": identity,
        "search_queries": [query],
        "required_terms": [[query]],
        "excluded_terms": [],
    }


def install_mock_client(monkeypatch, handler):
    transport = httpx.MockTransport(handler)
    monkeypatch.setattr(
        commands, "create_client", lambda settings: httpx.Client(transport=transport)
    )
    monkeypatch.setattr(
        commands, "create_requester", lambda client, settings: Requester(client, interval=0)
    )
    monkeypatch.setenv("INGESTION_TOKEN", "secret")


def partial_collection_handler(source_bytes, requests):
    def handler(request):
        requests.append(request)
        if request.url.path == "/api/v1/monitoring-targets":
            assert request.headers["authorization"] == "Bearer secret"
            return httpx.Response(
                200,
                json={"schema_version": "1.0", "targets": [target(), target("beans", "feijao")]},
            )
        assert "authorization" not in request.headers
        if request.url.params["ft"] == "arroz":
            return httpx.Response(206, content=source_bytes)
        return httpx.Response(403)

    return handler


def test_collect_checkpoints_before_later_source_failure(
    tmp_path, monkeypatch, source_bytes, capsys
):
    config = tmp_path / "config.toml"
    config.write_text('scraper_id="test"\n')
    requests = []
    install_mock_client(monkeypatch, partial_collection_handler(source_bytes, requests))
    monkeypatch.setattr(sys, "argv", ["scraper", "--config", str(config), "collect"])
    assert cli.main() == 1
    assert "complete=False" in capsys.readouterr().out
    assert len(requests) == 3
    with Spool(tmp_path / "data/spool.sqlite3") as spool:
        assert spool.status()[0] == 1
        submission = json.loads(list(spool.pending())[0][1])
        assert submission["observations"][0]["product"]["source_product_id"] == "307840"


def test_example_configuration_pins_curitiba_branch():
    settings = load_settings(Path(__file__).parents[1] / "muffato.example.toml")
    assert settings.sales_channel == "8"
    assert settings.city == "Curitiba"
    assert settings.state == "PR"
    assert settings.max_search_pages == 50
