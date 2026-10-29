"""Bounded catalog pagination; full pages at the limit are incomplete collections."""

import re
from datetime import UTC, datetime

from .fetching import request_products
from .json import decode_response
from .parsing import ParsedPage, parse_products

PAGE_SIZE = 50


def source_identities(products: list) -> frozenset[str]:
    try:
        return frozenset(sku["itemId"] for product in products for sku in product["items"])
    except (KeyError, TypeError) as error:
        raise ValueError("Malformed search products or SKU array") from error


def search_pages(requester, settings, target, query):
    seen_pages = set()
    page_number, offset, finished = 0, 0, False
    while page_number < settings.max_search_pages and not finished:
        page, offset, finished = fetch_search_page(
            requester, settings, target, query, offset, seen_pages
        )
        page_number += 1
        yield page
    if not finished:
        raise ValueError(
            "Search page limit reached; collection incomplete; narrow the target queries"
        )


def fetch_search_page(requester, settings, target, query, offset, seen_pages):
    response = request_products(requester, settings, query=query, offset=offset)
    timestamp = datetime.now(UTC)
    products = decode_response(response.content)
    validate_search_page(products, seen_pages)
    finished = page_finished(response, products, offset)
    page = ParsedPage()
    if products:
        page = parse_products(
            response.content,
            settings=settings,
            source_url=str(response.request.url),
            observed_at=timestamp,
            target=target,
        )
    return page, offset + len(products), finished


def page_finished(response, products: list, offset: int) -> bool:
    resources = response.headers.get("resources")
    if resources is None:
        return len(products) < PAGE_SIZE
    match = re.fullmatch(r"(\d+)-(\d+)/(\d+)", resources.strip())
    if match is None:
        raise ValueError("Malformed search resources header; collection incomplete")
    start, end, total = map(int, match.groups())
    if total == 0 and not products:
        return True
    if start != offset or min(end + 1, total) - start != len(products) or end < start:
        raise ValueError(
            f"Search resources {resources!r} do not match {len(products)} products "
            f"at offset {offset}; collection incomplete"
        )
    return offset + len(products) >= total


def validate_search_page(products, seen_pages: set) -> None:
    if not isinstance(products, list):
        raise ValueError("Expected a search product array")
    identities = source_identities(products)
    if products and (not identities or identities in seen_pages):
        raise ValueError("Search returned an empty or repeated SKU page; collection incomplete")
    seen_pages.add(identities)
