"""Browse regional category partitions within explicit page and request budgets."""

from collections.abc import Iterable, Iterator
from dataclasses import dataclass
from datetime import UTC, datetime
from uuid import uuid4

import httpx

from ..adapters import ParsedPage
from ..scope import MonitoringScope
from ..transport import Requester
from .config import VtexSettings
from .fetching import validate_response
from .json import decode_response
from .parsing import parse_products
from .search import validate_search_page

PAGE_SIZE = 50
MAX_SOURCE_PAGES = 50


@dataclass
class CatalogBudget:
    remaining: int

    def consume(self) -> None:
        if self.remaining == 0:
            raise ValueError("Catalog page limit reached; collection incomplete")
        self.remaining -= 1


def request_catalog(
    requester: Requester,
    settings: VtexSettings,
    page: int,
    count: int = PAGE_SIZE,
    facet_path: str = "",
) -> httpx.Response:
    suffix = "/" + facet_path if facet_path else ""
    response = requester.request(
        "GET",
        settings.shop_url + "/_v/api/intelligent-search/product_search" + suffix,
        params=catalog_parameters(settings, page, count),
        headers=settings.source_headers(),
    )
    validate_response(response)
    return response


def catalog_parameters(settings: VtexSettings, page: int, count: int) -> dict:
    return {
        "count": count,
        "page": page,
        "sc": settings.sales_channel,
        "_ir": str(uuid4()),
        **settings.region_parameters(),
    }


def catalog_pages(
    requester: Requester,
    settings: VtexSettings,
    scope: MonitoringScope,
    partitions: Iterable[str] = ("",),
) -> Iterator[ParsedPage]:
    if not scope.targets:
        return
    budget = CatalogBudget(settings.max_catalog_pages)
    for partition in partitions:
        yield from partition_pages(requester, settings, scope, partition, budget)


def partition_pages(requester, settings, scope, partition, budget):
    seen_pages = set()
    page_number, finished = 1, False
    while page_number <= MAX_SOURCE_PAGES and not finished:
        budget.consume()
        page, finished = fetch_catalog_page(
            requester, settings, scope, page_number, seen_pages, partition
        )
        page_number += 1
        yield page
    if not finished:
        raise ValueError("Category exceeds the source's 50-page limit; collection incomplete")


def fetch_catalog_page(requester, settings, scope, number, seen_pages, partition):
    response = request_catalog(requester, settings, number, facet_path=partition)
    timestamp = datetime.now(UTC)
    products, total = catalog_content(response.content)
    validate_search_page(products, seen_pages)
    finished = catalog_finished(products, total, number)
    return parse_catalog_page(response, settings, scope, timestamp, products), finished


def catalog_content(raw: bytes) -> tuple[list[dict], int]:
    data = decode_response(raw)
    if not isinstance(data, dict) or not isinstance(data.get("products"), list):
        raise ValueError("Malformed catalog response; collection incomplete")
    total = data.get("recordsFiltered")
    if type(total) is not int or total < 0:
        raise ValueError("Missing or invalid catalog total; collection incomplete")
    return data["products"], total


def catalog_finished(products, total: int, page: int) -> bool:
    offset = (page - 1) * PAGE_SIZE
    end = offset + len(products)
    if end > total or len(products) > PAGE_SIZE:
        raise ValueError("Catalog count disagrees with total; collection incomplete")
    if end >= total:
        return True
    if len(products) < PAGE_SIZE:
        raise ValueError("Short catalog page before total; collection incomplete")
    return False


def parse_catalog_page(response, settings, scope, timestamp, products):
    if not products:
        return ParsedPage()
    return parse_products(
        response.content,
        settings=settings,
        target=scope,
        source_url=str(response.request.url),
        observed_at=timestamp,
    )
