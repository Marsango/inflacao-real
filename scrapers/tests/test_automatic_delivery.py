from functools import partial
from uuid import uuid4

import httpx
import pytest

from inflacao_scrapers.delivery import Spool, batches, deliver
from inflacao_scrapers.delivery.client import DeliveryTarget
from inflacao_scrapers.delivery.immediate import DeliveryFailure, ImmediateDelivery
from inflacao_scrapers.transport import Requester


@pytest.fixture
def submission(observation):
    return next(
        batches([observation], scraper_id="scraper", shop_id="super-muffato", run_id=uuid4())
    )


@pytest.fixture
def destination():
    return DeliveryTarget("https://core.example", "secret-token", "scraper", "super-muffato")


def collect_batch(path, submission, destination, handler, attempts=3):
    with httpx.Client(transport=httpx.MockTransport(handler)) as client:
        requester = Requester(client, interval=0, sleep=lambda _: None, attempts=attempts)
        ImmediateDelivery(requester, destination, partial(Spool, path)).submit(submission)


@pytest.mark.parametrize("status", [200, 202])
def test_success_never_creates_local_storage(tmp_path, submission, destination, status):
    path = tmp_path / "data/spool.sqlite3"
    collect_batch(path, submission, destination, lambda request: httpx.Response(status))
    assert not path.parent.exists()


@pytest.mark.parametrize("status", [302, 400, 401, 403, 409, 413, 415, 429, 500, 503])
def test_rejected_submission_saves_exact_payload(tmp_path, submission, destination, status):
    requests = []

    def handler(request):
        requests.append(request)
        return httpx.Response(status, headers={"Location": "https://unexpected.example"})

    path = tmp_path / "spool.sqlite3"
    with pytest.raises(DeliveryFailure, match="saved locally"):
        collect_batch(path, submission, destination, handler)
    with Spool(path) as spool:
        assert list(spool.pending()) == [(str(submission.submission_id), submission.encode())]
    assert all(request.content == submission.encode() for request in requests)
    assert all(request.url.host == "core.example" for request in requests)
    assert len(requests) == (3 if status in (429, 500, 503) else 1)


def lost_response_handler(requests, recovery_status=None):
    def handler(request):
        requests.append(request)
        if recovery_status is not None and len(requests) > 1:
            return httpx.Response(recovery_status)
        raise httpx.ReadTimeout("secret-token: response lost after commit", request=request)

    return handler


def test_lost_response_retry_succeeds_without_spooling(tmp_path, submission, destination):
    requests = []
    path = tmp_path / "spool.sqlite3"
    collect_batch(path, submission, destination, lost_response_handler(requests, 200))
    assert len(requests) == 2
    assert requests[0].content == requests[1].content == submission.encode()
    assert not path.exists()


def test_lost_response_survives_restart_and_redelivers_identical_bytes(
    tmp_path, submission, destination, capsys
):
    requests = []
    path = tmp_path / "spool.sqlite3"
    with pytest.raises(DeliveryFailure) as failure:
        collect_batch(path, submission, destination, lost_response_handler(requests))
    assert len(requests) == 3
    with Spool(path) as spool:
        assert redeliver(spool, destination, requests) == 1
        assert spool.status() == (0, 0)
    assert len(requests) == 4
    assert all(request.content == submission.encode() for request in requests)
    assert "secret-token" not in str(failure.value)
    assert "secret-token" not in capsys.readouterr().out


def redeliver(spool, destination, requests):
    def handler(request):
        requests.append(request)
        return httpx.Response(200)

    with httpx.Client(transport=httpx.MockTransport(handler)) as client:
        return deliver(
            spool,
            Requester(client, interval=0),
            core_url=destination.core_url,
            token=destination.token,
            scraper_id=destination.scraper_id,
            shop_id=destination.shop_id,
        )


def test_unexpected_success_body_is_saved_for_investigation(tmp_path, submission, destination):
    path = tmp_path / "spool.sqlite3"
    with pytest.raises(DeliveryFailure):
        collect_batch(
            path, submission, destination, lambda request: httpx.Response(202, json={"ok": True})
        )
    with Spool(path) as spool:
        assert list(spool.pending()) == [(str(submission.submission_id), submission.encode())]
