"""Bounded public category browsing using the shared pacing/retry transport."""

from datetime import UTC, datetime
from urllib.parse import urlsplit

from .offers import CONTEXT_NOTE
from .pages import category_path, category_paths, parse_listing, product_identity
from .parsing import parse_cards


def fetch(requester, settings, path, redirects=0):
    response = requester.request(
        "GET", settings.shop_url + path, headers={"Cache-Control": "no-cache", "Pragma": "no-cache"}
    )
    if response.status_code in (301, 302, 303, 307, 308) and redirects < 3:
        destination = urlsplit(response.headers.get("Location", ""))
        if destination.scheme or destination.netloc or destination.query or destination.fragment:
            raise ValueError("Unexpected Sonda redirect destination")
        return fetch(requester, settings, category_path(destination.path), redirects + 1)
    if response.status_code != 200:
        raise ValueError(f"Sonda HTTP {response.status_code}; collection incomplete")
    return response.text, path


def catalog_pages(requester, settings, scope):
    paths = settings.category_paths or category_paths(fetch(requester, settings, "/delivery/")[0])
    budget = [settings.max_catalog_pages]
    for path in paths:
        yield from listing_pages(requester, settings, scope, path, budget)


def listing_pages(requester, settings, scope, path, budget):
    seen, expected_pages = set(), []
    while path is not None:
        consume_budget(budget)
        source, resolved_path = fetch(requester, settings, category_path(path))
        source_url = settings.shop_url + resolved_path
        cards, path = parse_listing(source, resolved_path, expected_pages)
        check_identities(cards, seen)
        yield parse_cards(cards, settings, source_url, datetime.now(UTC), scope)


def consume_budget(budget):
    if budget[0] <= 0:
        raise ValueError("Sonda catalog page limit reached; collection incomplete")
    budget[0] -= 1


def check_identities(cards, seen):
    identities = [product_identity(card) for card in cards]
    if len(set(identities)) != len(identities) or seen.intersection(identities):
        raise ValueError("Sonda repeated SKU/page; collection incomplete")
    seen.update(identities)


def prepare(requester, settings):
    print(f"shop=sonda: {CONTEXT_NOTE}", flush=True)
    return settings
