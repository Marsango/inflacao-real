"""Source-record regressions for Festval's regional kilogram and gum listings."""

from pathlib import Path

import pytest

from inflacao_scrapers.festval.config import FestvalSettings
from inflacao_scrapers.packaging import package_from_name
from inflacao_scrapers.vtex.json import decode_response

from .conftest import parse


@pytest.fixture
def festval_products():
    path = Path(__file__).parents[1] / "fixtures/stores/festval-size-policy.json"
    return decode_response(path.read_bytes())


@pytest.fixture
def festval_settings():
    return FestvalSettings(scraper_id="test", region_id="verified-test-region")


@pytest.mark.parametrize(
    "identity,amount",
    [
        ("2967", "14.99"),
        ("2901", "4.99"),
        ("2900", "7.89"),
        ("31332", "9.90"),
        ("26427", "4.99"),
        ("26475", "52.98"),
    ],
)
def test_stated_festval_weights_preserve_unscaled_prices(
    festval_products, festval_settings, identity, amount
):
    page = parse(festval_products, festval_settings, selected_sku=identity)
    assert len(page.observations) == 1
    measurement = page.observations[0]
    assert measurement.product.package is None
    assert measurement.offers[0].basis == "kg"
    assert measurement.offers[0].amount == amount
    assert len(measurement.offers) == 1
    assert all("payment-basis:" in issue for issue in page.warnings)


@pytest.mark.parametrize("multiplier", ["0", "-1", "10.01", "200"])
def test_unverified_festval_multipliers_remain_skipped(
    festval_products, festval_settings, multiplier
):
    product = next(item for item in festval_products if item["items"][0]["itemId"] == "2901")
    product["items"][0]["unitMultiplier"] = multiplier
    page = parse([product], festval_settings)
    assert not page.observations
    assert page.issues


@pytest.mark.parametrize("change", [{"city": "Other"}, {"seller_id": "other"}, {"region_id": None}])
def test_larger_festval_weights_require_verified_context(
    festval_products, festval_settings, change
):
    settings = festval_settings.model_copy(update=change)
    page = parse(festval_products, settings, selected_sku="2901")
    assert not page.observations
    assert page.issues


def test_mentos_marketing_line_does_not_become_liters(festval_products, festval_settings):
    page = parse(festval_products, festval_settings, selected_sku="28432")
    assert len(page.observations) == 1
    assert not page.issues
    package = page.observations[0].product.package
    assert package.quantity == "8.5"
    assert package.unit == "g"
    assert page.observations[0].offers[0].amount == "3.99"


def test_unrelated_liter_quantities_remain_measurements():
    package = package_from_name("Água Mineral 3L")
    assert package.quantity == "3"
    assert package.unit == "l"


def test_contradictory_bean_weights_retain_price_without_package(
    festval_products, festval_settings
):
    page = parse(festval_products, festval_settings, selected_sku="3468")
    assert len(page.observations) == 1
    assert page.observations[0].product.package is None
    assert not page.issues
    assert any("ambiguous package" in issue for issue in page.warnings)
