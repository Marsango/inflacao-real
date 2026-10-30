"""Retailer searches require URI spaces, not form-encoded plus signs."""

import httpx
import pytest

from inflacao_scrapers.festval.config import FestvalSettings
from inflacao_scrapers.transport import Requester
from inflacao_scrapers.vtex.fetching import request_products


@pytest.mark.parametrize(
    "query", ["batata doce", "creme dental", "pão de forma", "arroz + feijão & café"]
)
def test_search_encodes_spaces_without_changing_query(settings, query):
    def handler(request):
        assert b"+" not in request.url.query
        assert b"%20" in request.url.query
        assert request.url.params["ft"] == query
        assert request.url.params["sc"] == "8"
        assert request.url.params["_from"] == "50"
        assert request.url.params["_to"] == "99"
        return httpx.Response(200, json=[])

    with httpx.Client(transport=httpx.MockTransport(handler)) as client:
        response = request_products(Requester(client), settings, query=query, offset=50)
    assert response.status_code == 200


def test_search_preserves_festval_region_and_segment():
    settings = FestvalSettings(scraper_id="collector", region_id="region+with/slash==")

    def handler(request):
        assert request.url.params["regionId"] == settings.region_id
        assert request.url.params["sc"] == "2"
        assert request.url.params["ft"] == "creme dental"
        assert b"%2B" in request.url.query
        assert "vtex_segment=" in request.headers["cookie"]
        return httpx.Response(200, json=[])

    with httpx.Client(transport=httpx.MockTransport(handler)) as client:
        request_products(Requester(client), settings, query="creme dental")


def test_search_preserves_explicit_sku_selection(settings):
    def handler(request):
        assert request.url.params["fq"] == "skuId:307840"
        assert "ft" not in request.url.params
        assert "_ir" in request.url.params
        return httpx.Response(206, json=[])

    with httpx.Client(transport=httpx.MockTransport(handler)) as client:
        request_products(Requester(client), settings, sku="307840")
