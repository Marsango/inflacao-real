"""Bounded public searches filtered against the core's monitoring targets."""

from datetime import UTC, datetime

from .config import PAGE_SIZE
from .pagination import validate_page
from .parsing import parse_products
from .session import request_document, verify_context


def search_pages(requester, settings, target, query):
    settings.require_context()
    seen, expected_total, number, finished = set(), None, 0, False
    while not finished:
        start = number * PAGE_SIZE
        document, url = fetch_grid(requester, settings, query, number)
        observed_at = datetime.now(UTC)
        finished, expected_total = validate_page(document, query, start, seen, expected_total)
        verify_context(requester, settings)
        yield parse_products(document, settings, url, observed_at, target)
        number += 1


def fetch_grid(requester, settings, query, number):
    if number >= settings.max_search_pages:
        raise ValueError("Nagumo search page limit reached; collection incomplete")
    return request_document(
        requester,
        settings,
        "Search-UpdateGrid",
        {"q": query, "start": number * PAGE_SIZE, "sz": PAGE_SIZE},
    )
