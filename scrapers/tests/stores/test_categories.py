import json

import httpx
import pytest

from inflacao_scrapers.angeloni.categories import catalog_partitions
from inflacao_scrapers.transport import Requester


def partitions(sample, identifiers=None):
    settings = sample.prepared_settings()
    if identifiers is not None:
        settings = settings.model_copy(update={"catalog_category_ids": identifiers})
    transport = httpx.MockTransport(lambda request: httpx.Response(200, content=sample.categories))
    with httpx.Client(transport=transport) as client:
        return catalog_partitions(Requester(client), settings)


def test_verified_retail_leaf_categories_resolve_without_keyword_queries(samples):
    paths = partitions(samples["angeloni"])
    assert len(paths) == 39
    assert paths[0] == "category-1/2/category-2/43"
    assert "category-1/2/category-2/250/category-3/274" in paths


def test_new_upstream_category_is_discovered_without_configuration_changes(samples):
    sample = samples["angeloni"]
    categories = json.loads(sample.categories)
    categories.append(
        {"id": 900001, "name": "New department", "hasChildren": False, "children": []}
    )
    sample.categories = json.dumps(categories).encode()
    assert "category-1/900001" in partitions(sample)


def test_optional_selection_restricts_diagnostic_browsing(samples):
    assert partitions(samples["angeloni"], [43]) == ["category-1/2/category-2/43"]


def test_empty_category_tree_cannot_report_complete_collection(samples):
    sample = samples["angeloni"]
    sample.categories = b"[]"
    with pytest.raises(ValueError, match="no leaf categories"):
        partitions(sample)


def test_truncated_category_tree_cannot_silently_drop_a_department(samples):
    sample = samples["angeloni"]
    categories = json.loads(sample.categories)
    categories.append({"id": 900001, "name": "Truncated", "hasChildren": True, "children": []})
    sample.categories = json.dumps(categories).encode()
    with pytest.raises(ValueError, match="incomplete tree"):
        partitions(sample)


def test_added_foods_have_resolvable_catalog_partitions(samples):
    paths = partitions(samples["angeloni"])
    assert "category-1/82/category-2/83" in paths
    assert "category-1/153/category-2/187" in paths
    assert "category-1/153/category-2/190" in paths
    assert "category-1/138/category-2/144" in paths
    assert "category-1/138/category-2/149" in paths
    assert "category-1/2/category-2/256/category-3/322" in paths
    assert "category-1/2/category-2/256/category-3/324" in paths


def test_household_essentials_have_resolvable_catalog_partitions(samples):
    paths = partitions(samples["angeloni"])
    assert "category-1/152/category-2/163" in paths
    assert "category-1/152/category-2/166" in paths
    assert "category-1/152/category-2/159/category-3/168" in paths
    assert "category-1/152/category-2/161/category-3/173" in paths
    assert "category-1/195/category-2/196" in paths
    assert "category-1/195/category-2/205" in paths
    assert "category-1/195/category-2/207" in paths


def test_missing_configured_category_cannot_silently_reduce_coverage(samples):
    with pytest.raises(ValueError, match="categories missing"):
        partitions(samples["angeloni"], [999999])


def test_large_parent_categories_must_be_split_into_leaf_categories(samples):
    with pytest.raises(ValueError, match="leaf catalog categories"):
        partitions(samples["angeloni"], [2])
