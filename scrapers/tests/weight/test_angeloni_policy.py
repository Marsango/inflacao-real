"""Regression cases from Angeloni's Curitiba catalog on 2026-10-30."""

from pathlib import Path

import pytest

from inflacao_scrapers.angeloni.config import AngeloniSettings
from inflacao_scrapers.packaging import package_from_name
from inflacao_scrapers.vtex.json import decode_response

from .conftest import parse


@pytest.fixture
def angeloni_products():
    path = Path(__file__).parents[1] / "fixtures/stores/angeloni-size-policy.json"
    return decode_response(path.read_bytes())


@pytest.fixture
def angeloni_settings():
    return AngeloniSettings(scraper_id="test", region_id="verified-test-region")


@pytest.mark.parametrize("identity,weight", [("4764846", "60"), ("5086142", "45")])
def test_nutrition_claims_do_not_conflict_with_net_weight(
    angeloni_products, angeloni_settings, identity, weight
):
    page = parse(angeloni_products, angeloni_settings, selected_sku=identity)
    assert len(page.observations) == 1
    assert not page.issues
    package = page.observations[0].product.package
    assert package.quantity == weight
    assert package.unit == "g"


@pytest.mark.parametrize("name", ["Barra 20g de proteína 60g", "Barra 60g 20g de proteína"])
def test_protein_claim_is_ignored_regardless_of_position(name):
    assert package_from_name(name).quantity == "60"


def test_ambiguous_pringles_listing_retains_unknown_package(angeloni_products, angeloni_settings):
    page = parse(angeloni_products, angeloni_settings, selected_sku="5523711")
    assert len(page.observations) == 1
    assert page.observations[0].product.package is None
    assert not page.issues
    assert any("ambiguous package" in issue for issue in page.warnings)


def test_watermelon_preserves_per_kilogram_price(angeloni_products, angeloni_settings):
    page = parse(angeloni_products, angeloni_settings, selected_sku="8117")
    assert len(page.observations) == 1
    assert not page.issues
    measurement = page.observations[0]
    assert measurement.product.package is None
    assert measurement.product.gtin is None
    assert measurement.offers[0].basis == "kg"
    assert measurement.offers[0].amount == "4.79"
    assert len(measurement.offers) == 1


def test_implausible_ham_multiplier_is_skipped(angeloni_products, angeloni_settings):
    page = parse(angeloni_products, angeloni_settings, selected_sku="4706969")
    assert not page.observations
    assert any("unsupported unit multiplier" in issue for issue in page.issues)


@pytest.mark.parametrize("multiplier", ["5.01", "200"])
def test_larger_angeloni_multipliers_stay_unverified(
    angeloni_products, angeloni_settings, multiplier
):
    product = next(item for item in angeloni_products if item["productId"] == "8117")
    product["items"][0]["unitMultiplier"] = multiplier
    page = parse([product], angeloni_settings)
    assert not page.observations
    assert page.issues
