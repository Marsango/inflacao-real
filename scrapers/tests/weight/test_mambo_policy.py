"""Mambo price bases are enabled only for the verified São Paulo delivery context."""

import json
from pathlib import Path

import pytest

from inflacao_scrapers.mambo.config import MamboSettings
from inflacao_scrapers.vtex.json import decode_response

from .conftest import parse, source_offer


@pytest.fixture
def mambo_products():
    path = Path(__file__).parents[1] / "fixtures/stores/mambo-weight-candidates.json"
    return decode_response(path.read_bytes())


@pytest.fixture
def mambo_settings():
    return MamboSettings(scraper_id="test", region_id="verified-test-region")


def test_mambo_prices_match_saved_independent_rendered_kilogram_labels(
    mambo_products, mambo_settings
):
    path = Path(__file__).parents[1] / "fixtures/stores/mambo-weight-verification.json"
    evidence = json.loads(path.read_bytes())
    page = parse(mambo_products, mambo_settings)
    observations = {item.product.source_product_id: item for item in page.observations}
    assert evidence["storefront_context_verified"] is True
    assert len(evidence["rendered_candidates"]) == len(observations) == 5
    for record in evidence["rendered_candidates"]:
        amount = observations[record["sku"]].offers[0].amount
        label = "R$ " + amount.replace(".", ",") + "/Kg"
        assert label in record["visible_excerpt"].replace("\u00a0", " ")
        assert record["postal_code"] == mambo_settings.postal_code
        assert record["channel"] == "delivery"


@pytest.mark.parametrize(
    "change",
    [{"shop_url": "https://other.example"}, {"city": "Other"}, {"state": "PR"},
     {"postal_code": "01310100"}, {"sales_channel": "2"}, {"seller_id": "other"},
     {"regional_seller_id": "other"}, {"region_id": None}],
)
def test_unverified_mambo_context_cannot_produce_kilogram_prices(
    mambo_products, mambo_settings, change
):
    settings = mambo_settings.model_copy(update=change)
    page = parse(mambo_products, settings)
    assert not page.observations
    assert len(page.issues) == len(mambo_products)


@pytest.mark.parametrize("multiplier", ["0", "-1", "3.01", "200"])
def test_mambo_weight_bounds_do_not_expand_beyond_verified_portions(
    mambo_products, mambo_settings, multiplier
):
    product = mambo_products[0]
    product["items"][0]["unitMultiplier"] = multiplier
    page = parse([product], mambo_settings)
    assert not page.observations
    assert page.issues


def test_unavailable_mambo_weight_placeholder_never_becomes_a_zero_price(
    mambo_products, mambo_settings
):
    product = mambo_products[0]
    source_offer(product).update(Price=0, ListPrice=0, IsAvailable=False, AvailableQuantity=0)
    page = parse([product], mambo_settings)
    assert not page.issues
    observation = page.observations[0]
    assert observation.availability == "out_of_stock"
    assert observation.offers == []


@pytest.mark.parametrize("change", [{"isKit": True}, {"measurementUnit": "g"}])
def test_mambo_kits_and_other_weight_units_remain_explicitly_unsupported(
    mambo_products, mambo_settings, change
):
    product = mambo_products[0]
    product["items"][0].update(change)
    page = parse([product], mambo_settings)
    assert not page.observations
    assert page.issues
