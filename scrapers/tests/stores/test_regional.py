import base64
import json

import httpx
import pytest

from inflacao_scrapers.transport import Requester
from inflacao_scrapers.vtex.regional import prepare_region


def test_region_is_resolved_from_cep_and_expected_seller(store_sample):
    requests = []

    def handler(request):
        requests.append(request)
        return httpx.Response(200, content=store_sample.regions)

    with httpx.Client(transport=httpx.MockTransport(handler)) as client:
        settings = prepare_region(Requester(client), store_sample.settings)
    assert requests[0].url.params["postalCode"] == "80730201"
    assert requests[0].url.params["sc"] == store_sample.settings.sales_channel
    assert store_sample.settings.regional_seller_id in settings.store_id
    assert "region:" in settings.store_id
    assert "authorization" not in requests[0].headers


def test_prepared_source_headers_preserve_public_region_without_core_credentials(store_sample):
    settings = store_sample.prepared_settings()
    headers = settings.source_headers()
    cookie = headers["Cookie"].removeprefix("vtex_segment=")
    segment = json.loads(base64.b64decode(cookie))
    assert segment["regionId"] == settings.region_id
    assert segment["channel"] == settings.sales_channel
    assert "authorization" not in {key.lower() for key in headers}
    assert headers["Cache-Control"] == "no-cache, max-age=0"


def test_wrong_regional_seller_cannot_be_labeled_as_curitiba(store_sample):
    settings = store_sample.settings.model_copy(update={"regional_seller_id": "wrong-branch"})

    def handler(request):
        return httpx.Response(200, content=store_sample.regions)

    with (
        httpx.Client(transport=httpx.MockTransport(handler)) as client,
        pytest.raises(ValueError, match="seller absent"),
    ):
        prepare_region(Requester(client), settings)


def test_unprepared_delivery_context_cannot_fetch_prices(store_sample):
    with pytest.raises(ValueError, match="must be verified"):
        store_sample.settings.source_headers()


@pytest.mark.parametrize("body", [b"[]", b"{}", b'[{"id":"region","sellers":[]}]'])
def test_unusable_region_responses_stop_collection(samples, body):
    def handler(request):
        return httpx.Response(200, content=body)

    with httpx.Client(transport=httpx.MockTransport(handler)) as client, pytest.raises(ValueError):
        prepare_region(Requester(client), samples["angeloni"].settings)
