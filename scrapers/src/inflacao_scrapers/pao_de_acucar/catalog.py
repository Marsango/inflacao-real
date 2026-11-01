"""Bounded category browsing with explicit incomplete-run failures."""

from datetime import UTC, datetime

from .parsing import parse_products, product_identity
from .source import request_document

PAGE_SIZE = 12


def categories(requester, settings):
    document, _ = request_document(
        requester,
        "category/ecom/categories",
        parameters={"storeId": settings.store_id, "showSub": "true"},
    )
    entries = document.get("content")
    if not isinstance(entries, list) or not entries:
        raise ValueError("GPA category tree missing")
    paths = [category_path(entry) for entry in entries]
    if len(set(paths)) != len(paths) or any(not path for path in paths):
        raise ValueError("GPA category paths missing or repeated")
    return paths


def category_path(entry):
    path = entry["uiLink"]
    if not isinstance(path, str) or not path.startswith("/"):
        raise ValueError("Invalid GPA category path")
    path = path.strip("/")
    return path.replace("/", "|")


def catalog_pages(requester, settings, scope):
    settings.require_context()
    budget = [settings.max_catalog_pages]
    for path in categories(requester, settings):
        yield from category_pages(requester, settings, scope, path, budget)


def category_pages(requester, settings, scope, path, budget):
    seen, expected_total = set(), None
    number, finished = 1, False
    while not finished:
        consume_budget(budget)
        document, url = fetch_page(requester, settings, path, number)
        finished, expected_total = validate_page(document, number, seen, expected_total)
        yield parse_products(document, settings, url, datetime.now(UTC), scope)
        number += 1


def consume_budget(budget):
    if budget[0] <= 0:
        raise ValueError("GPA catalog page limit reached; collection incomplete")
    budget[0] -= 1


def fetch_page(requester, settings, path, number):
    body = {
        "partner": "linx",
        "page": number,
        "resultsPerPage": PAGE_SIZE,
        "multiCategory": path,
        "sortBy": "relevance",
        "department": "ecom",
        "storeId": int(settings.store_id),
        "customerPlus": True,
        "filters": [],
    }
    return request_document(requester, "search/category-page", body=body)


def pagination_values(document, number):
    values = [document.get(key) for key in ("page", "totalPages", "totalProducts")]
    if any(type(value) is not int for value in values):
        raise ValueError("Invalid GPA pagination values")
    page, pages, total = values
    if page != number or total < 0 or pages != max(1, (total + PAGE_SIZE - 1) // PAGE_SIZE):
        raise ValueError("Inconsistent GPA pagination totals")
    return pages, total


def validate_page(document, number, seen, expected_total):
    pages, total = pagination_values(document, number)
    products = document.get("products")
    if not isinstance(products, list):
        raise ValueError("GPA product array missing")
    if expected_total is not None and total != expected_total:
        raise ValueError("GPA catalog changed during pagination; collection incomplete")
    identities = [product_identity(product) for product in products]
    if len(set(identities)) != len(identities) or seen.intersection(identities):
        raise ValueError("GPA repeated product/page; collection incomplete")
    if len(products) != min(PAGE_SIZE, max(0, total - (number - 1) * PAGE_SIZE)):
        raise ValueError("GPA short/oversized page; collection incomplete")
    seen.update(identities)
    return number >= pages, total
