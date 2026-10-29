from datetime import UTC, datetime

import httpx
import pytest

from inflacao_scrapers.angeloni.categories import catalog_partitions
from inflacao_scrapers.scope import MonitoringTarget
from inflacao_scrapers.transport import Requester
from inflacao_scrapers.vtex.catalog import catalog_content, request_catalog
from inflacao_scrapers.vtex.fetching import request_products
from inflacao_scrapers.vtex.parsing import parse_products
from inflacao_scrapers.vtex.search import source_identities


def live_response(requester, settings, name):
    if name == "angeloni":
        paths = catalog_partitions(requester, settings)
        return request_catalog(requester, settings, page=1, count=2, facet_path=paths[0])
    return request_products(requester, settings, query="arroz", limit=5)


def live_target(name):
    if name == "angeloni":
        return None
    return MonitoringTarget(
        id="rice",
        name="Rice",
        search_queries=["arroz"],
        required_terms=[["arroz"]],
        excluded_terms=["kit"],
    )


@pytest.mark.live
def test_live_regional_prices_and_parser(store_sample):
    with httpx.Client(timeout=25, follow_redirects=False) as client:
        requester = Requester(client)
        settings = store_sample.adapter.prepare(requester, store_sample.settings)
        response = live_response(requester, settings, store_sample.name)
    page = parse_products(
        response.content,
        settings=settings,
        source_url=str(response.request.url),
        observed_at=datetime.now(UTC),
        target=live_target(store_sample.name),
    )
    assert not page.issues, page.issues
    assert page.observations
    assert all(
        observation.context.postal_code == settings.postal_code for observation in page.observations
    )


@pytest.mark.live
def test_live_angeloni_catalog_pagination(samples):
    sample = samples["angeloni"]
    with httpx.Client(timeout=25, follow_redirects=False) as client:
        requester = Requester(client)
        settings = sample.adapter.prepare(requester, sample.settings)
        first, second, last, number = pagination_responses(requester, settings)
    first_products, first_total = catalog_content(first.content)
    second_products, second_total = catalog_content(second.content)
    assert first_total > 4
    assert second_total > 4
    assert len(first_products) == len(second_products) == 2
    assert source_identities(first_products).isdisjoint(source_identities(second_products))
    last_products, last_total = catalog_content(last.content)
    assert (number - 1) * 50 + len(last_products) == last_total


def pagination_responses(requester, settings):
    facet_path = catalog_partitions(requester, settings)[0]
    first = request_catalog(requester, settings, page=1, count=2, facet_path=facet_path)
    second = request_catalog(requester, settings, page=2, count=2, facet_path=facet_path)
    _, total = catalog_content(first.content)
    number = (total + 49) // 50
    assert number <= 50
    last = request_catalog(requester, settings, page=number, count=50, facet_path=facet_path)
    return first, second, last, number


@pytest.mark.live
def test_live_angeloni_food_partitions_fit_source_limits(samples):
    sample = samples["angeloni"]
    with httpx.Client(timeout=25, follow_redirects=False) as client:
        requester = Requester(client)
        settings = sample.adapter.prepare(requester, sample.settings)
        verify_partition_limits(requester, settings)


def verify_partition_limits(requester, settings):
    for facet_path in catalog_partitions(requester, settings):
        response = request_catalog(requester, settings, page=1, count=1, facet_path=facet_path)
        _, total = catalog_content(response.content)
        assert total <= 2500, f"Category {facet_path} exceeds the source's 50-page window"


@pytest.mark.live
@pytest.mark.parametrize(
    "name,selection,sku",
    [
        ("angeloni", 183, "150177"),
        ("angeloni", 84, "4721796"),
        ("angeloni", 226, "58297"),
        ("festval", "banana", "7515"),
        ("festval", "patinho", "5568"),
        ("festval", "pao", "2164"),
    ],
)
def test_live_regional_advertised_kilogram_prices(samples, name, selection, sku):
    sample = samples[name]
    with httpx.Client(timeout=25, follow_redirects=False) as client:
        requester = Requester(client)
        settings = sample.adapter.prepare(requester, sample.settings)
        response = regional_weight_response(requester, settings, name, selection)
    page = parse_products(
        response.content,
        settings=settings,
        source_url=str(response.request.url),
        observed_at=datetime.now(UTC),
        selected_sku=sku,
    )
    assert_regional_weight_page(page, settings)


def regional_weight_response(requester, settings, name, selection):
    if name == "angeloni":
        settings = settings.model_copy(update={"catalog_category_ids": [selection]})
        paths = catalog_partitions(requester, settings)
        return request_catalog(requester, settings, page=1, count=10, facet_path=paths[0])
    return request_products(requester, settings, query=selection, limit=10)


def assert_regional_weight_page(page, settings):
    assert len(page.observations) == 1, page.issues
    assert all("payment-basis:" in issue for issue in page.issues), page.issues
    observation = page.observations[0]
    assert observation.offers, "Missing advertised kilogram price; live verification failed"
    assert all(offer.basis == "kg" for offer in observation.offers)
    assert observation.product.package is None
    assert observation.context.city == "Curitiba"
    assert observation.context.state == "PR"
    assert observation.context.postal_code == settings.postal_code
    assert settings.regional_seller_id in observation.context.store_id
