"""Bounded department browsing; search routes excluded by robots are never used."""

from datetime import UTC, datetime

from .config import CATALOG_PREFIX
from .parsing import parse_products
from .session import departments, request_document
from .values import product_identity


def catalog_pages(requester, settings, scope):
    budget = [settings.max_catalog_pages]
    for department in departments(requester, settings):
        yield from department_pages(requester, settings, scope, department, budget)


def department_pages(requester, settings, scope, department, budget):
    path = CATALOG_PREFIX + f"classificacoes_mercadologicas/departamentos/{department}/produtos"
    yield from listing_pages(requester, settings, scope, path, budget)


def listing_pages(requester, settings, scope, path, budget):
    seen, expected_total = set(), None
    number, finished = 1, False
    while not finished:
        consume_budget(budget)
        document, url = request_document(requester, settings, path, {"page": number})
        finished, expected_total = validate_page(document, number, seen, expected_total)
        yield parse_products(document, settings, url, datetime.now(UTC), scope)
        number += 1


def consume_budget(budget):
    if budget[0] <= 0:
        raise ValueError("Hirota catalog page limit reached; collection incomplete")
    budget[0] -= 1


def validate_page(document, number, seen, expected_total):
    products, pagination = document.get("data"), document.get("paginator")
    if not isinstance(products, list) or not isinstance(pagination, dict):
        raise ValueError("Missing Hirota pagination; collection incomplete")
    total, size, pages = pagination_values(pagination, number)
    if expected_total is not None and total != expected_total:
        raise ValueError("Hirota catalog changed during pagination; collection incomplete")
    identities = [product_identity(product) for product in products]
    if len(set(identities)) != len(identities) or seen.intersection(identities):
        raise ValueError("Hirota repeated product/page; collection incomplete")
    if len(products) != min(size, max(0, total - (number - 1) * size)):
        raise ValueError("Hirota short/oversized page; collection incomplete")
    seen.update(identities)
    return number >= pages, total


def pagination_values(pagination, number):
    values = [
        pagination.get(key) for key in ("page", "items_per_page", "total_pages", "total_items")
    ]
    if any(type(value) is not int for value in values):
        raise ValueError("Invalid Hirota pagination values")
    page, size, pages, total = values
    if (
        page != number
        or not 1 <= size <= 100
        or total < 0
        or pages != max(1, (total + size - 1) // size)
    ):
        raise ValueError("Inconsistent Hirota pagination totals")
    return total, size, pages
