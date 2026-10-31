import json
import sys
from pathlib import Path

import httpx
import pytest

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


def scope_response(include_failure):
    targets = [target()]
    if include_failure:
        targets.append(target("beans", "feijao"))
    return httpx.Response(200, json={"schema_version": "1.0", "targets": targets})


def collection_handler(source_bytes, requests, *, delivery_status=202, include_failure=True):
    def handler(request):
        requests.append(request)
        if request.url.path == "/api/v1/monitoring-targets":
            assert request.headers["authorization"] == "Bearer secret"
            return scope_response(include_failure)
        if request.method == "POST":
            assert request.headers["authorization"] == "Bearer secret"
            return httpx.Response(delivery_status)
        assert "authorization" not in request.headers
        if request.url.params["ft"] == "arroz":
            return httpx.Response(206, content=source_bytes)
        return httpx.Response(403)

    return handler


def test_collect_delivers_before_later_source_failure(tmp_path, monkeypatch, source_bytes, capsys):
    config = tmp_path / "config.toml"
    config.write_text('scraper_id="test"\n')
    requests = []
    install_mock_client(monkeypatch, collection_handler(source_bytes, requests))
    monkeypatch.setattr(sys, "argv", ["scraper", "--config", str(config), "collect"])
    assert cli.main() == 1
    assert "complete=False" in capsys.readouterr().out
    assert len(requests) == 4
    assert requests[2].method == "POST"
    assert requests[2].url.path == "/api/v1/ingestion/submissions"
    assert not (tmp_path / "data").exists()


@pytest.mark.parametrize("status", [200, 202])
def test_collect_command_delivers_without_creating_storage(
    tmp_path, monkeypatch, source_bytes, capsys, status
):
    config = tmp_path / "config.toml"
    config.write_text('scraper_id="test"\n')
    requests = []
    handler = collection_handler(
        source_bytes, requests, delivery_status=status, include_failure=False
    )
    install_mock_client(monkeypatch, handler)
    monkeypatch.setattr(sys, "argv", ["scraper", "--config", str(config), "collect"])
    assert cli.main() == 0
    assert len(requests) == 3
    assert requests[-1].method == "POST"
    assert "complete=True" in capsys.readouterr().out
    assert not (tmp_path / "data").exists()


def test_collect_command_saves_failed_delivery_at_configured_path(
    tmp_path, monkeypatch, source_bytes, capsys
):
    config = tmp_path / "config.toml"
    config.write_text('scraper_id="test"\nspool_path="failed/batches.sqlite3"\n')
    requests = []
    handler = collection_handler(source_bytes, requests, delivery_status=401)
    install_mock_client(monkeypatch, handler)
    monkeypatch.setattr(sys, "argv", ["scraper", "--config", str(config), "collect"])
    assert cli.main() == 1
    assert len(requests) == 3
    with Spool(tmp_path / "failed/batches.sqlite3") as spool:
        identity, payload = next(spool.pending())
        assert payload == requests[-1].content
        assert identity == json.loads(payload)["submission_id"]
    output = capsys.readouterr()
    assert "HTTP 401" in output.err
    assert "secret" not in output.err
    assert "complete=False" in output.out


def test_example_configuration_pins_curitiba_branch():
    settings = load_settings(Path(__file__).parents[1] / "muffato.example.toml")
    assert settings.sales_channel == "8"
    assert settings.city == "Curitiba"
    assert settings.state == "PR"
    assert settings.max_search_pages == 50


def test_discovery_reports_metadata_warnings_without_failure(capsys):
    from inflacao_scrapers.adapters import ParsedPage

    page = ParsedPage(warnings=["barcode: unknown GTIN"])
    assert commands.report_discovery(page) == 0
    assert "warning: barcode: unknown GTIN" in capsys.readouterr().err
