import json
from datetime import UTC, datetime

import pytest
from stores.conftest import FIXTURES

from inflacao_scrapers.sonda.config import SondaSettings
from inflacao_scrapers.sonda.html import class_nodes, suffix_nodes
from inflacao_scrapers.sonda.offers import money
from inflacao_scrapers.sonda.pages import parse_listing
from inflacao_scrapers.sonda.parsing import parse_cards

PATH = "/delivery/categoria/Mercearia"
NOW = datetime(2026, 11, 1, 12, tzinfo=UTC)


def source(name="catalog"):
    return (FIXTURES / f"sonda-{name}.html").read_text()


def settings(**values):
    return SondaSettings(scraper_id="collector-sp-01", **values)


def parse(name="catalog"):
    evidence = json.loads((FIXTURES / "sonda-verification.json").read_text())
    url = evidence["sources"][name]["url"]
    cards, _ = parse_listing(source(name), url.removeprefix(settings().shop_url))
    return parse_cards(cards, settings(), url, NOW)


def test_visible_prices_override_conflicting_template_metadata_and_keep_source_sku():
    result = parse()
    assert not result.issues
    assert result.observations[0].offers[0].amount == "27.16"
    product = result.observations[2]
    assert product.product.source_product_id == "593311"
    assert product.product_url.endswith("/1000004434")
    assert product.product.gtin is None
    assert product.product.package.quantity == "400"
    assert product.product.package.unit == "g"


def test_fulfillment_unknowns_and_online_channel_remain_explicit():
    observation = parse().observations[0]
    assert observation.collection_method == "html" and observation.observed_at == NOW
    assert observation.context.channel == "online"
    context = observation.context.model_dump(exclude={"channel"})
    assert all(value is None for value in context.values())
    conditions = observation.offers[0].conditions
    assert conditions.payment_method == "unknown" and conditions.membership_required is None
    assert conditions.coupon_required is None and "não verificada" in conditions.description


def test_regular_discounts_preserve_labels_without_inventing_reference_prices():
    offers = [item.offers[0] for item in parse().observations if item.offers]
    assert any("OFF" in offer.conditions.description for offer in offers)
    assert all(offer.reference_amount is None for offer in offers)


@pytest.mark.parametrize("name", ["rice", "beans", "milk", "coffee", "cleaning"])
def test_captured_food_and_cleaning_prices(name):
    # Category excerpts retain their actual pagination paths.
    evidence = json.loads((FIXTURES / "sonda-verification.json").read_text())
    url = evidence["sources"][name]["url"]
    path = url.removeprefix(settings().shop_url)
    cards, _ = parse_listing(source(name), path)
    result = parse_cards(cards, settings(), url, NOW)
    assert not result.issues and result.observations
    assert result.observations[0].product.package is not None


def test_variable_weight_totals_are_never_relabelled_per_kg():
    result = parse("weight")
    assert result.issues and not result.observations
    assert all("variable-weight" in issue for issue in result.issues)


@pytest.mark.parametrize("value", [None, "0,00"])
def test_synthetic_unknown_and_zero_price_do_not_become_zero_offers(value):
    cards, _ = parse_listing(source(), PATH)
    price = suffix_nodes(cards[0], "_lblValorPor")[0]
    price.children = [] if value is None else [value]
    if value is None:
        price.attributes["id"] = "missing"
    result = parse_cards(cards[:1], settings(), settings().shop_url + PATH, NOW)
    assert not result.issues and not result.observations[0].offers


def test_synthetic_unavailable_product_has_no_offer():
    cards, _ = parse_listing(source(), PATH)
    suffix_nodes(cards[0], "_lnkAddToCarrinho")[0].attributes["id"] = "missing"
    suffix_nodes(cards[0], "_pnlValor")[0].attributes["id"] = "test_pnlIndisponivel"
    result = parse_cards(cards[:1], settings(), settings().shop_url + PATH, NOW)
    assert result.observations[0].availability == "out_of_stock"
    assert not result.observations[0].offers


def test_synthetic_quantity_or_membership_promotion_is_not_unconditional():
    cards, _ = parse_listing(source(), PATH)
    class_nodes(cards[0], "product--badges")[0].children = ["Leve 3 pague 2 Cartão Sonda"]
    result = parse_cards(cards[:1], settings(), settings().shop_url + PATH, NOW)
    assert result.issues and not result.observations


@pytest.mark.parametrize("value", ["NaN", "1.23", "-1,00", "1,001", "1.00,00", "R$ 1,00"])
def test_invalid_monetary_formats_are_rejected(value):
    with pytest.raises(ValueError):
        money(value)
    assert money("1.234,56") == "1234.56"


@pytest.mark.parametrize(
    "values",
    [{"store_id": "1"}, {"city": "São Paulo"}, {"state": "SP"}, {"postal_code": "01310200"}],
)
def test_configuration_cannot_falsely_claim_a_selected_context(values):
    with pytest.raises(ValueError):
        settings(**values)
