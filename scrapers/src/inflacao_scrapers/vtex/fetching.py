"""Paced, explicitly regionalized source requests without result caching."""

from datetime import UTC, datetime
from urllib.parse import quote, urlencode
from uuid import uuid4

import httpx

from ..transport import Requester
from .config import VtexSettings
from .parsing import parse_products


def search_parameters(
    settings: VtexSettings, sku: str | None, query: str | None, limit: int, offset: int
) -> dict:
    parameters = {
        "sc": settings.sales_channel,
        "_from": str(offset),
        "_to": str(offset + limit - 1),
        "_ir": str(uuid4()),
    }
    parameters.update(settings.region_parameters())
    if sku is not None:
        return {**parameters, "fq": "skuId:" + sku}
    if query:
        return {**parameters, "ft": query}
    raise ValueError("Specify a query or SKU")


def search_url(settings, sku, query, limit, offset) -> str:
    parameters = search_parameters(settings, sku, query, limit, offset)
    encoded = urlencode(parameters, quote_via=quote)
    return settings.shop_url + "/api/catalog_system/pub/products/search?" + encoded


def validate_response(response: httpx.Response) -> None:
    if response.status_code not in (200, 206):
        raise ValueError(f"Source HTTP {response.status_code}; collection incomplete")
    if int(response.headers.get("Age", "0")) > 0:
        raise ValueError("Source returned an aged cached response; collection incomplete")


def request_products(
    requester: Requester,
    settings: VtexSettings,
    *,
    sku: str | None = None,
    query: str | None = None,
    limit: int = 50,
    offset: int = 0,
) -> httpx.Response:
    response = requester.request(
        "GET",
        search_url(settings, sku, query, limit, offset),
        headers=settings.source_headers(),
    )
    validate_response(response)
    return response


def fetch_products(
    requester: Requester, settings: VtexSettings, **selection
) -> tuple[bytes, str, datetime]:
    response = request_products(requester, settings, **selection)
    observed_at = datetime.now(UTC)
    return response.content, str(response.request.url), observed_at


def discover(requester, settings, query, limit):
    raw, url, timestamp = fetch_products(requester, settings, query=query, limit=limit)
    return parse_products(raw, settings=settings, source_url=url, observed_at=timestamp)
