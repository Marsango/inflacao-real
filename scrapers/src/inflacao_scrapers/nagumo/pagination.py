"""Check grid identity, counts, page progression and the site's next link."""

from urllib.parse import parse_qs, urlsplit

from .config import PAGE_SIZE, ROUTE_PREFIX
from .values import product_identity


def validate_page(document, query, start, seen, expected_total):
    products, pagination = document.get("productsSearchResult"), document.get("productSearch")
    if not isinstance(products, list) or not isinstance(pagination, dict):
        raise ValueError("Missing Nagumo grid/pagination; collection incomplete")
    total = pagination_total(document, pagination, query, start)
    if expected_total is not None and total != expected_total:
        raise ValueError("Nagumo result count changed during pagination; collection incomplete")
    identities = [product_identity(product) for product in products]
    if len(set(identities)) != len(identities) or seen.intersection(identities):
        raise ValueError("Nagumo repeated product/page; collection incomplete")
    if len(products) != min(PAGE_SIZE, max(0, total - start)):
        raise ValueError("Nagumo short/oversized page; collection incomplete")
    finished = start + len(products) >= total
    validate_next(pagination.get("showMoreUrl"), query, start + PAGE_SIZE, finished)
    seen.update(identities)
    return finished, total


def pagination_total(document, pagination, query, start):
    values = [
        document.get("count"),
        pagination.get("count"),
        pagination.get("pageNumber"),
        pagination.get("pageSize"),
    ]
    if any(type(value) is not int for value in values):
        raise ValueError("Invalid Nagumo pagination values")
    total, nested_total, page, size = values
    if total < 0 or total != nested_total or page != start // PAGE_SIZE or size != PAGE_SIZE:
        raise ValueError("Inconsistent Nagumo pagination counts")
    if pagination.get("searchKeywords") != query or pagination.get("isCategorySearch") is not False:
        raise ValueError("Nagumo grid does not match the requested search")
    return total


def validate_next(value, query, next_start, finished):
    if finished and value == "finished":
        return
    if finished or not isinstance(value, str):
        raise ValueError("Nagumo next-page link inconsistent with result count")
    parts = urlsplit(value)
    expected = {"q": [query], "start": [str(next_start)], "sz": [str(PAGE_SIZE)]}
    if parts.scheme != "https" or parts.netloc != "www.nagumo.com.br" or parts.fragment:
        raise ValueError("Untrusted Nagumo next-page link")
    if parts.path != ROUTE_PREFIX + "Search-UpdateGrid" or parse_qs(parts.query) != expected:
        raise ValueError("Nagumo next-page link did not advance the same search")
