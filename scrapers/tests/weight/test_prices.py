from decimal import Decimal

import pytest

from inflacao_scrapers.angeloni.config import AngeloniSettings
from inflacao_scrapers.festval.config import FestvalSettings

from .conftest import parse, rename, source_offer


def test_observed_produce_beef_bakery_and_fixed_control(weighted_products, settings):
    page = parse(weighted_products, settings)
    assert len(page.observations) == 4
    assert len(page.warnings) == 3
    assert all("payment-basis" in issue for issue in page.warnings)
    for observation in page.observations[:3]:
        assert_weight_price(observation)
    control = page.observations[3]
    assert control.product.package.quantity == "320"
    assert control.offers[0].basis == "package"
    assert control.offers[0].amount == "26.80"


def assert_weight_price(observation):
    expected = {"186092": "13.40", "269830": "58.70", "145168": "20.50"}
    assert observation.product.package is None
    assert observation.product.gtin is None
    assert len(observation.offers) == 1
    offer = observation.offers[0]
    assert offer.amount == expected[observation.product.source_product_id]
    assert offer.basis == "kg"
    assert offer.reference_amount == offer.amount
    assert offer.conditions.minimum_quantity is None
    assert "peso final" in offer.conditions.description


def test_approximate_weight_and_reference_name_never_become_package(weighted_products, settings):
    rename(weighted_products[0], "Banana 1kg")
    observation = parse(weighted_products[:1], settings).observations[0]
    assert observation.product.package is None
    assert observation.offers[0].amount == "13.40"
    assert "Peso aprox. 150g/un" in observation.offers[0].conditions.description


@pytest.mark.parametrize("multiplier", [None, "NaN", "Infinity", True, 0, -1, 2])
def test_invalid_multipliers_are_explicit_issues(weighted_products, settings, multiplier):
    weighted_products[0]["items"][0]["unitMultiplier"] = multiplier
    page = parse(weighted_products[:1], settings)
    assert not page.observations
    assert "shop=super-muffato sku=186092" in page.issues[0]


@pytest.mark.parametrize("change", [{"measurementUnit": "g"}, {"isKit": True}, {"isKit": None}])
def test_unsupported_formats_keep_valid_neighbors(weighted_products, settings, change):
    weighted_products[0]["items"][0].update(change)
    page = parse(weighted_products, settings)
    assert len(page.observations) == 3
    assert "sale-format" in page.issues[0]


@pytest.mark.parametrize("model", [AngeloniSettings, FestvalSettings])
def test_unprepared_regional_settings_never_enable_weight_mapping(weighted_products, model):
    settings = model(scraper_id="test")
    page = parse(weighted_products[:1], settings)
    assert not page.observations
    assert "unverified" in page.issues[0]


@pytest.mark.parametrize("change", [{"sales_channel": "1"}, {"seller_id": "other"}])
def test_other_contexts_are_not_enabled_by_accident(weighted_products, settings, change):
    page = parse(weighted_products[:1], settings.model_copy(update=change))
    assert not page.observations
    assert "unverified" in page.issues[0]


@pytest.mark.parametrize(
    "change,availability",
    [({"Price": None}, "in_stock"), ({"Price": 0, "IsAvailable": False}, "out_of_stock")],
)
def test_unknown_prices_remain_empty_offers(weighted_products, settings, change, availability):
    source_offer(weighted_products[0]).update(change)
    page = parse(weighted_products[:1], settings)
    assert not page.issues
    assert page.observations[0].offers == []
    assert page.observations[0].availability == availability


def test_unavailable_displayed_price_survives(weighted_products, settings):
    source_offer(weighted_products[0]).update(IsAvailable=False, AvailableQuantity=0)
    observation = parse(weighted_products[:1], settings).observations[0]
    assert observation.availability == "out_of_stock"
    assert observation.offers[0].amount == "13.40"


def test_unresolved_reference_is_omitted_without_losing_price(weighted_products, settings):
    source_offer(weighted_products[0])["ListPrice"] = Decimal("2.01")
    page = parse(weighted_products[:1], settings)
    assert page.observations[0].offers[0].reference_amount is None
    assert page.observations[0].offers[0].amount == "13.40"
    assert any("reference-price" in issue for issue in page.warnings)


def test_promotions_preserve_unknown_thresholds(weighted_products, settings):
    source_offer(weighted_products[0])["Teasers"] = [{"name": "Clube: leve 0,5 kg"}]
    conditions = parse(weighted_products[:1], settings).observations[0].offers[0].conditions
    assert "Clube: leve 0,5 kg" in conditions.description
    assert conditions.minimum_quantity is None
    assert conditions.membership_required is None


def test_malformed_gtin_is_not_repaired(weighted_products, settings):
    weighted_products[0]["items"][0]["ean"] = "7894904296300"
    page = parse(weighted_products[:1], settings)
    assert len(page.observations) == 1
    assert page.observations[0].product.gtin is None
    assert "GTIN" in page.warnings[0]


def test_each_collection_uses_fresh_identity_and_time(weighted_products, settings):
    first = parse(weighted_products[:1], settings).observations[0]
    second = parse(weighted_products[:1], settings).observations[0]
    assert first.product.source_product_id == second.product.source_product_id
    assert first.observation_id != second.observation_id
    assert first.observed_at < second.observed_at


@pytest.mark.parametrize(
    "options", ["malformed", {"installmentOptions": "malformed"}, {"installmentOptions": [None]}]
)
def test_invalid_payment_options_keep_fixed_package_base_price(
    weighted_products, settings, options
):
    source_offer(weighted_products[3])["PaymentOptions"] = options
    page = parse(weighted_products[3:], settings)
    assert len(page.observations[0].offers) == 1
    assert page.observations[0].offers[0].amount == "26.80"
    assert any("payment-offer" in issue for issue in page.warnings)


def test_one_bad_payment_does_not_discard_independent_alternatives(weighted_products, settings):
    source = source_offer(weighted_products[3])
    source["PaymentOptions"]["installmentOptions"][0]["installments"][0]["total"] = "invalid"
    page = parse(weighted_products[3:], settings)
    assert any("payment-offer" in issue for issue in page.warnings)
    assert page.observations[0].offers[0].amount == "26.80"
    assert page.observations[0].offers[1].conditions.payment_method == "pix"
