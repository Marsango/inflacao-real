"""Paced, explicitly regionalized source requests without result caching."""

from datetime import UTC, datetime
from uuid import uuid4

import httpx

from ..config import Settings
from ..transport import Requester


def search_parameters(
    settings: Settings, sku: str | None, query: str | None, limit: int, offset: int
) -> dict:
    parameters = {
        "sc": settings.sales_channel,
        "_from": str(offset),
        "_to": str(offset + limit - 1),
        "_ir": str(uuid4()),
    }
    if sku is not None:
        return {**parameters, "fq": "skuId:" + sku}
    if query:
        return {**parameters, "ft": query}
    raise ValueError("Specify a query or SKU")


def validate_response(response: httpx.Response) -> None:
    if response.status_code not in (200, 206):
        raise ValueError(f"Source HTTP {response.status_code}; collection incomplete")
    if int(response.headers.get("Age", "0")) > 0:
        raise ValueError("Source returned an aged cached response; collection incomplete")


def request_products(
    requester: Requester,
    settings: Settings,
    *,
    sku: str | None = None,
    query: str | None = None,
    limit: int = 50,
    offset: int = 0,
) -> httpx.Response:
    response = requester.request(
        "GET",
        settings.shop_url + "/api/catalog_system/pub/products/search",
        params=search_parameters(settings, sku, query, limit, offset),
        headers={"Cache-Control": "no-cache, max-age=0", "Pragma": "no-cache"},
    )
    validate_response(response)
    return response


def fetch_products(
    requester: Requester, settings: Settings, **selection
) -> tuple[bytes, str, datetime]:
    response = request_products(requester, settings, **selection)
    observed_at = datetime.now(UTC)
    return response.content, str(response.request.url), observed_at
