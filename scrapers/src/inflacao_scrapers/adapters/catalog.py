"""Recover independent departments without hiding source or pagination failures."""

import httpx

from ..transport import RequestFailure
from . import ParsedPage

SOURCE_ERRORS = (ValueError, RequestFailure, httpx.HTTPError)


def collect_categories(paths, fetch_pages, shop_id):
    for path in paths:
        yield from collect_category(path, fetch_pages, shop_id)


def collect_category(path, fetch_pages, shop_id):
    print(f"shop={shop_id} category={path} starting", flush=True)
    try:
        yield from fetch_pages(path)
    except SOURCE_ERRORS as error:
        yield ParsedPage(issues=[f"shop={shop_id} category={path}: {error}"])
        return
    print(f"shop={shop_id} category={path} traversal_finished", flush=True)


def check_page_identities(identities, seen, shop):
    unique = set(identities)
    if len(unique) != len(identities) or (unique and unique.issubset(seen)):
        raise ValueError(f"{shop} repeated product/page; collection incomplete")
    overlap = len(unique.intersection(seen))
    seen.update(unique)
    return overlap


def describe_page(page, location, overlap):
    page.issues = [f"{location}: {issue}" for issue in page.issues]
    if overlap:
        message = f"{location}: overlapping_listings={overlap}; first measurement retained"
        page.warnings.append(message)
    return page
