import httpx
import pytest

from inflacao_scrapers.history_refresh import request_history_refresh
from inflacao_scrapers.transport import Requester


def test_refresh_is_authenticated_and_has_no_observations(settings):
    requests = []

    def handler(request):
        requests.append(request)
        return httpx.Response(202)

    with httpx.Client(transport=httpx.MockTransport(handler)) as client:
        request_history_refresh(Requester(client, interval=0), settings, "test")
    assert len(requests) == 1
    assert requests[0].url.path == "/api/v1/ingestion/history-refresh"
    assert requests[0].headers["authorization"] == "Bearer test"
    assert b'"scraper_id"' in requests[0].content
    assert b'"observations"' not in requests[0].content


@pytest.mark.parametrize("status", [401, 403, 500])
def test_refresh_failure_does_not_change_collection_status(settings, capsys, status):
    with httpx.Client(transport=httpx.MockTransport(lambda _: httpx.Response(status))) as client:
        request_history_refresh(Requester(client, interval=0, attempts=1), settings, "secret")
    assert "notification failed" in capsys.readouterr().err

