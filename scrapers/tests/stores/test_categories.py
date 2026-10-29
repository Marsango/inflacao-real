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


def test_verified_food_leaf_categories_resolve_without_keyword_queries(samples):
    paths = partitions(samples["angeloni"])
    assert len(paths) == 15
    assert paths[0] == "category-1/2/category-2/43"
    assert "category-1/2/category-2/250/category-3/274" in paths


def test_missing_configured_category_cannot_silently_reduce_coverage(samples):
    with pytest.raises(ValueError, match="categories missing"):
        partitions(samples["angeloni"], [999999])


def test_large_parent_categories_must_be_split_into_leaf_categories(samples):
    with pytest.raises(ValueError, match="leaf catalog categories"):
        partitions(samples["angeloni"], [2])
