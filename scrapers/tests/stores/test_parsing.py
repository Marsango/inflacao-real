from datetime import UTC, datetime
from decimal import Decimal

import pytest

from inflacao_scrapers.scope import MonitoringScope, MonitoringTarget
from inflacao_scrapers.vtex.offers import offers_from_source
from inflacao_scrapers.vtex.parsing import parse_products

from .conftest import FIXTURES, encode_source


def parse(sample, raw=None, target=None):
    return parse_products(
        raw or sample.source,
        settings=sample.prepared_settings(),
        source_url=sample.settings.shop_url + "/catalog",
        observed_at=datetime(2026, 10, 29, 18, tzinfo=UTC),
        target=target,
    )


def test_saved_store_responses_preserve_exact_money_and_region(store_sample):
    page = parse(store_sample)
    assert not page.issues, page.issues
    assert len(page.observations) == {"angeloni": 2, "festval": 3}[store_sample.name]
    observation = page.observations[0]
    assert (
        observation.offers[0].amount == {"angeloni": "7.49", "festval": "14.98"}[store_sample.name]
    )
    assert observation.context.postal_code == "80730201"
    assert observation.context.channel == "delivery"
    assert observation.context.city == "Curitiba"
    assert store_sample.settings.regional_seller_id in observation.context.store_id
    assert observation.product.gtin
    assert any(item.product.package for item in page.observations)
    assert isinstance(
        store_sample.products()[0]["items"][0]["sellers"][0]["commertialOffer"]["Price"], Decimal
    )


def test_modern_catalog_stock_uses_available_quantity_and_absolute_product_url(samples):
    page = parse(samples["angeloni"])
    assert all(observation.availability == "in_stock" for observation in page.observations)
    assert all(
        observation.product_url.startswith("https://super.angeloni.com.br/")
        for observation in page.observations
    )


def test_festval_retains_unavailable_offers_and_source_promotions(samples):
    page = parse(samples["festval"])
    unavailable = page.observations[-1]
    assert unavailable.availability == "out_of_stock"
    assert unavailable.offers[0].amount == "27.96"
    assert unavailable.product.source_product_id == "7291"
    assert page.observations[0].availability == "in_stock"


def test_missing_stock_and_zero_placeholder_never_create_free_price():
    assert offers_from_source({"Price": 0, "AvailableQuantity": 0}) == []


def test_filters_run_before_unsupported_variable_weight_parsing(store_sample):
    products = store_sample.products()
    products[0]["items"][0].update(measurementUnit="kg", nameComplete="Unrelated produce")
    target = MonitoringTarget(
        id="rice",
        name="Rice",
        search_queries=["arroz"],
        required_terms=[["arroz", "cerveja"]],
        excluded_terms=[],
    )
    scope = MonitoringScope(schema_version="1.0", targets=[target])
    page = parse(store_sample, encode_source(products), scope)
    assert not page.issues
    assert len(page.observations) >= 1


def test_each_measurement_gets_fresh_identity(store_sample):
    first = parse(store_sample).observations[0]
    second = parse(store_sample).observations[0]
    assert first.observation_id != second.observation_id


def test_unprepared_context_is_reported_as_an_extraction_failure(store_sample):
    page = parse_products(
        store_sample.source,
        settings=store_sample.settings,
        source_url=store_sample.settings.shop_url,
        observed_at=datetime.now(UTC),
    )
    assert not page.observations
    assert page.issues
    assert "must be verified" in page.issues[0]


def test_actual_regional_weight_candidates_preserve_advertised_bases(store_sample):
    raw = (FIXTURES / f"{store_sample.name}-weight-candidates.json").read_bytes()
    page = parse(store_sample, raw)
    assert len(page.observations) == {"angeloni": 4, "festval": 5}[store_sample.name]
    assert all("payment-basis:" in issue for issue in page.warnings), page.issues
    weighted = [item for item in page.observations if item.offers[0].basis == "kg"]
    assert len(weighted) == {"angeloni": 3, "festval": 4}[store_sample.name]
    assert all(item.product.package is None and item.product.gtin is None for item in weighted)
    assert all(item.context.city == "Curitiba" for item in weighted)
    assert all(item.context.postal_code == "80730201" for item in weighted)
    assert all(
        store_sample.settings.regional_seller_id in item.context.store_id for item in weighted
    )


def test_regional_weight_amounts_are_not_scaled_by_portion_size(store_sample):
    raw = (FIXTURES / f"{store_sample.name}-weight-candidates.json").read_bytes()
    page = parse(store_sample, raw)
    expected = {
        "150177": "6.59",
        "4721796": "49.49",
        "58297": "17.99",
        "7515": "7.99",
        "5568": "76.98",
        "5621": "69.98",
        "2164": "36.90",
    }
    for item in page.observations:
        assert_regional_amount(item, expected)


def assert_regional_amount(item, expected):
    identity = item.product.source_product_id
    if identity not in expected:
        return
    assert len(item.offers) == 1
    assert item.offers[0].amount == expected[identity]
    assert item.offers[0].basis == "kg"


@pytest.mark.parametrize(
    "change",
    [
        {"city": "Other city"},
        {"state": "SC"},
        {"sales_channel": "99"},
        {"seller_id": "other"},
        {"regional_seller_id": "other"},
        {"region_id": None},
    ],
)
def test_regional_weight_prices_require_known_curitiba_context(store_sample, change):
    raw = (FIXTURES / f"{store_sample.name}-weight-candidates.json").read_bytes()
    settings = store_sample.prepared_settings().model_copy(update=change)
    page = parse_products(
        raw, settings=settings, source_url=settings.shop_url, observed_at=datetime.now(UTC)
    )
    assert page.issues
    assert any("sale-format: unverified" in issue for issue in page.issues)
    assert all(offer.basis == "package" for item in page.observations for offer in item.offers)
