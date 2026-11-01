import json
import sys

import httpx
from stores.test_pipeline import install_mock_runtime, monitoring_response, submissions

from inflacao_scrapers import cli
from inflacao_scrapers.delivery import Spool, deliver
from inflacao_scrapers.transport import Requester

from .test_catalog import last_page
from .test_parsing import source


def execute(tmp_path, monkeypatch, fail=False):
    requests = []
    configuration = tmp_path / "sonda.toml"
    configuration.write_text(
        'adapter="sonda"\nscraper_id="collector-sp-01"\n'
        'category_paths=["/delivery/categoria/Mercearia"]\n'
    )
    monkeypatch.setenv("INGESTION_TOKEN", "core-secret")
    install_mock_runtime(monkeypatch, responder(requests, fail))
    monkeypatch.setattr(sys, "argv", ["scraper", "--config", str(configuration), "collect"])
    return cli.main(), requests


def responder(requests, fail):
    def respond(request):
        requests.append(request)
        if request.url.host != "127.0.0.1":
            assert "authorization" not in request.headers
            return httpx.Response(200, text=last_page(source()))
        assert request.headers["authorization"] == "Bearer core-secret"
        if request.method == "GET":
            scope = json.loads(monitoring_response().content)
            scope["targets"][0]["required_terms"] = [["abacaxi"]]
            return httpx.Response(200, json=scope)
        return httpx.Response(401 if fail else 202)

    return respond


def test_shared_collection_delivers_unknown_context_and_reports_scope(
    tmp_path, monkeypatch, capsys
):
    status, requests = execute(tmp_path, monkeypatch)
    assert status == 0
    document = submissions(requests)[0]
    assert document["shop_id"] == document["collector"]["name"] == "sonda"
    assert document["collector"]["version"]
    assert document["observations"]
    assert all(item["context"]["channel"] == "online" for item in document["observations"])
    assert all(item["context"]["city"] is None for item in document["observations"])
    assert "target=pantry matched_listings=" in capsys.readouterr().out
    assert not (tmp_path / "data").exists()


def test_failed_delivery_spools_identical_payload_and_retries(tmp_path, monkeypatch):
    status, requests = execute(tmp_path, monkeypatch, fail=True)
    assert status == 1
    attempted = next(request.content for request in requests if request.method == "POST")
    retry_spool(tmp_path / "data/spool.sqlite3", attempted)


def retry_spool(path, attempted):
    received = []
    with httpx.Client(transport=httpx.MockTransport(lambda r: accept(r, received))) as client:
        with Spool(path) as spool:
            assert next(spool.pending())[1] == attempted
            count = deliver(
                spool,
                Requester(client, interval=0),
                core_url="http://127.0.0.1:8080",
                token="secret",
                scraper_id="collector-sp-01",
                shop_id="sonda",
            )
            assert count == 1 and spool.status() == (0, 0)
    assert received == [attempted]


def accept(request, received):
    received.append(request.content)
    return httpx.Response(200)


def test_saved_submission_validates_against_ingestion_v1():
    from stores.conftest import FIXTURES

    from inflacao_scrapers.models import Submission

    saved = Submission.model_validate_json((FIXTURES / "sonda-submission.json").read_bytes())
    assert saved.shop_id == "sonda"
    assert all(item.context.channel == "online" for item in saved.observations)
    assert all(item.context.store_id is None for item in saved.observations)
