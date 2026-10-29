import copy

import httpx
import pytest

from inflacao_scrapers.scope import MonitoringScope, MonitoringTarget
from inflacao_scrapers.transport import Requester
from inflacao_scrapers.vtex.catalog import catalog_pages

from .conftest import encode_source


def scope():
    target = MonitoringTarget(
        id="chocolate",
        name="Chocolate",
        search_queries=["chocolate"],
        required_terms=[["chocolate"]],
        excluded_terms=[],
    )
    return MonitoringScope(schema_version="1.0", targets=[target])


def products(sample, count, offset=0):
    template = sample.products()[0]
    return [product_with_identity(template, position) for position in range(offset, offset + count)]


def product_with_identity(template, identity):
    result = copy.deepcopy(template)
    result["productId"] = str(identity)
    result["items"][0]["itemId"] = str(identity)
    return result


def browse(sample, pages, limit=1000):
    requests = []

    def handler(request):
        requests.append(request)
        page = pages[int(request.url.params["page"])]
        return httpx.Response(200, content=encode_source(page))

    settings = sample.prepared_settings().model_copy(update={"max_catalog_pages": limit})
    with httpx.Client(transport=httpx.MockTransport(handler)) as client:
        result = list(catalog_pages(Requester(client, interval=0), settings, scope()))
    return result, requests


def test_catalog_paginates_once_and_avoids_disallowed_keyword_parameters(samples):
    sample = samples["angeloni"]
    pages = {
        1: {"products": products(sample, 50), "recordsFiltered": 51},
        2: {"products": products(sample, 1, 50), "recordsFiltered": 51},
    }
    result, requests = browse(sample, pages)
    assert sum(len(page.observations) for page in result) == 51
    assert [request.url.params["page"] for request in requests] == ["1", "2"]
    assert all(
        not {"ft", "fq", "query", "map"}.intersection(request.url.params) for request in requests
    )
    assert all(
        request.url.params["regionId"] == sample.prepared_settings().region_id
        for request in requests
    )
    assert all("vtex_segment=" in request.headers["cookie"] for request in requests)


def test_repeated_catalog_page_cannot_report_success(samples):
    sample = samples["angeloni"]
    page = {"products": products(sample, 50), "recordsFiltered": 100}
    with pytest.raises(ValueError, match="repeated"):
        browse(sample, {1: page, 2: page})


def test_catalog_page_budget_is_explicit_failure(samples):
    sample = samples["angeloni"]
    page = {"products": products(sample, 50), "recordsFiltered": 51}
    with pytest.raises(ValueError, match="page limit"):
        browse(sample, {1: page}, limit=1)


def test_short_page_cannot_silently_skip_listings(samples):
    sample = samples["angeloni"]
    page = {"products": products(sample, 2), "recordsFiltered": 51}
    with pytest.raises(ValueError, match="Short catalog"):
        browse(sample, {1: page})


def test_missing_catalog_total_is_not_success(samples):
    with pytest.raises(ValueError, match="catalog total"):
        browse(samples["angeloni"], {1: {"products": []}})
