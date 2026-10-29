from datetime import UTC, datetime
from uuid import uuid4

import httpx
import pytest

from inflacao_scrapers.delivery import MAX_BYTES, Spool, batches, deliver
from inflacao_scrapers.models import Observation, Submission
from inflacao_scrapers.transport import Requester, RequestFailure


def batch(observations):
    return list(
        batches(observations, scraper_id="scraper", shop_id="super-muffato", run_id=uuid4())
    )


def fresh(observation):
    document = observation.model_dump()
    document["observation_id"] = uuid4()
    return Observation.model_validate(document)


def drain(spool, handler, **options):
    with httpx.Client(transport=httpx.MockTransport(handler)) as client:
        return deliver(
            spool,
            Requester(client, interval=0, sleep=lambda _: None, **options),
            core_url="https://core.example",
            token="secret-token",
            scraper_id="scraper",
            shop_id="super-muffato",
            report=lambda _: None,
        )


def test_batch_count_limits_and_identity(observation):
    observations = [fresh(observation) for _ in range(101)]
    result = batch(observations)
    assert [len(submission.observations) for submission in result] == [100, 1]
    assert len({submission.submission_id for submission in result}) == 2
    assert [
        observation.observation_id
        for submission in result
        for observation in submission.observations
    ] == [observation.observation_id for observation in observations]
    assert all(len(submission.encode()) <= MAX_BYTES for submission in result)


def test_batches_count_utf8_bytes_including_envelope(observation):
    document = observation.model_dump()
    # Valid large observations force the byte limit before the count limit.
    document["offers"] = [
        {
            **observation.offers[0].model_dump(),
            "amount": f"{position + 1}.00",
            "reference_amount": None,
            "conditions": {
                **observation.offers[0].conditions.model_dump(),
                "description": "ã" * 2048,
            },
        }
        for position in range(20)
    ]
    large = Observation.model_validate(document)
    result = batch([fresh(large) for _ in range(20)])
    assert len(result) > 1
    assert sum(len(submission.observations) for submission in result) == 20
    assert all(len(submission.encode()) <= MAX_BYTES for submission in result)


def test_spool_survives_restart_without_changing_payload(tmp_path, observation):
    submission = batch([observation])[0]
    path = tmp_path / "spool.sqlite3"
    with Spool(path) as spool:
        spool.enqueue(submission)
        spool.enqueue(submission)
        assert spool.status()[0] == 1
    with Spool(path) as spool:
        identity, payload = list(spool.pending())[0]
        assert payload == submission.encode()
        assert identity == str(submission.submission_id)
        changed = Submission.model_validate(
            {**submission.model_dump(), "created_at": datetime.now(UTC)}
        )
        assert_replacement_rejected(spool, changed)
        assert list(spool.pending())[0][1] == payload


def test_spool_capacity_does_not_discard_queued_batches(tmp_path, observation):
    submission = batch([observation])[0]
    with Spool(tmp_path / "spool.sqlite3", capacity=len(submission.encode())) as spool:
        spool.enqueue(submission)
        assert_spool_full(spool, batch([fresh(observation)])[0])
        assert spool.status()[0] == 1


def test_lost_response_redelivery_reuses_exact_bytes(tmp_path, observation):
    submission = batch([observation])[0]
    requests = []

    def handler(request):
        requests.append(request)
        if len(requests) == 1:
            raise httpx.ReadTimeout("Response lost after server commit", request=request)
        return httpx.Response(200)

    with Spool(tmp_path / "spool.sqlite3") as spool:
        spool.enqueue(submission)
        assert drain(spool, handler) == 1
        assert spool.status()[0] == 0
    assert requests[0].content == requests[1].content == submission.encode()
    assert requests[0].headers["authorization"] == "Bearer secret-token"


@pytest.mark.parametrize("status", [200, 202])
def test_empty_success_acknowledges_batch(tmp_path, observation, status):
    with Spool(tmp_path / "spool.sqlite3") as spool:
        spool.enqueue(batch([observation])[0])
        assert drain(spool, lambda request: httpx.Response(status)) == 1
        assert spool.status() == (0, 0)


@pytest.mark.parametrize("status", [400, 401, 403, 409, 413, 415, 302])
def test_rejected_or_redirected_submissions_remain_queued(tmp_path, observation, status):
    requests = []

    def handler(request):
        requests.append(request)
        return httpx.Response(status, headers={"Location": "https://unexpected.example"})

    with Spool(tmp_path / "spool.sqlite3") as spool:
        spool.enqueue(batch([observation])[0])
        assert_delivery_rejected(spool, handler, match=f"HTTP {status}")
        assert spool.status()[0] == 1
    assert len(requests) == 1


def test_failed_delivery_restarts_from_unchanged_spool(tmp_path, observation):
    submission = batch([observation])[0]
    path = tmp_path / "spool.sqlite3"
    with Spool(path) as spool:
        spool.enqueue(submission)
        assert_delivery_rejected(
            spool, lambda request: httpx.Response(503), RequestFailure, attempts=2
        )
    with Spool(path) as spool:
        assert list(spool.pending())[0][1] == submission.encode()
        assert drain(spool, lambda request: httpx.Response(202)) == 1


def test_nonempty_success_body_is_not_acknowledged(tmp_path, observation):
    with Spool(tmp_path / "spool.sqlite3") as spool:
        spool.enqueue(batch([observation])[0])
        assert_delivery_rejected(
            spool, lambda request: httpx.Response(202, json={"accepted": True})
        )
        assert spool.status()[0] == 1


def test_retry_after_budget_is_respected():
    sleeps = []
    requests = []

    def handler(request):
        requests.append(request)
        return httpx.Response(429, headers={"Retry-After": "120"})

    with httpx.Client(transport=httpx.MockTransport(handler)) as client:
        assert_retry_budget(client, sleeps)
    assert len(requests) == 1
    assert not sleeps


def test_no_credentials_in_delivery_log(tmp_path, observation):
    with Spool(tmp_path / "spool.sqlite3") as spool:
        spool.enqueue(batch([observation])[0])
        failure = assert_delivery_rejected(spool, lambda request: httpx.Response(401))
        assert "secret-token" not in str(failure)


def test_empty_collection_never_creates_empty_submission():
    assert batch([]) == []


def assert_replacement_rejected(spool, changed):
    with pytest.raises(ValueError, match="immutable"):
        spool.enqueue(changed)


def assert_spool_full(spool, submission):
    with pytest.raises(RuntimeError, match="exhausted"):
        spool.enqueue(submission)


def assert_delivery_rejected(spool, handler, exception=RuntimeError, match=None, **options):
    with pytest.raises(exception, match=match) as failure:
        drain(spool, handler, **options)
    return failure.value


def assert_retry_budget(client, sleeps):
    requester = Requester(client, retry_max=60, sleep=sleeps.append)
    with pytest.raises(RequestFailure, match="budget"):
        requester.request("GET", "https://shop.example")
