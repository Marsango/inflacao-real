from copy import deepcopy

import pytest

from inflacao_scrapers.nagumo.pagination import validate_page
from inflacao_scrapers.nagumo.session import validate_branches

from .conftest import source


def test_captured_first_second_last_and_empty_pages_have_consistent_counts():
    seen = set()
    assert validate_page(source("rice"), "arroz", 0, seen, None) == (False, 577)
    assert validate_page(source("rice-next"), "arroz", 24, seen, 577) == (False, 577)
    assert validate_page(source("rice-last"), "arroz", 576, seen, 577) == (True, 577)
    assert validate_page(source("empty"), "nossainflacaoinexistente", 0, set(), None) == (True, 0)


def test_synthetic_repeat_cannot_be_successful_exhaustion():
    seen = set()
    validate_page(source("rice"), "arroz", 0, seen, None)
    repeated = source("rice-next")
    repeated["productsSearchResult"] = source("rice")["productsSearchResult"]
    with pytest.raises(ValueError, match="repeated"):
        validate_page(repeated, "arroz", 24, seen, 577)


@pytest.mark.parametrize(
    "field,value",
    [
        ("count", -1),
        ("pageSize", 25),
        ("pageNumber", True),
        ("searchKeywords", "leite"),
        ("showMoreUrl", "https://evil.example/"),
    ],
)
def test_synthetic_malformed_grid_metadata_never_completes(field, value):
    document = source("rice")
    document["productSearch"][field] = value
    with pytest.raises(ValueError):
        validate_page(document, "arroz", 0, set(), None)


def test_synthetic_short_page_and_changed_count_are_failures():
    document = source("rice")
    document["productsSearchResult"].pop()
    with pytest.raises(ValueError, match="short"):
        validate_page(document, "arroz", 0, set(), None)
    with pytest.raises(ValueError, match="changed"):
        validate_page(source("rice"), "arroz", 0, set(), 576)


def test_captured_context_is_operational_and_unique():
    validate_branches(source("context"), require_active=True)
    document = source("context")
    document["stores"].append(
        deepcopy(next(store for store in document["stores"] if store["active"]))
    )
    with pytest.raises(ValueError, match="ambiguous"):
        validate_branches(document, require_active=True)


@pytest.mark.parametrize(
    "change",
    [
        {"operation": False},
        {"city": "SANTO ANDRE"},
        {"address": "another address"},
        {"active": False},
    ],
)
def test_synthetic_changed_or_lost_pickup_context_stops_collection(change):
    document = source("context")
    branch = next(store for store in document["stores"] if store["active"])
    branch.update(change)
    with pytest.raises(ValueError):
        validate_branches(document, require_active=True)
