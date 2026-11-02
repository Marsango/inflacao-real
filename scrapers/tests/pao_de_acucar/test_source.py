from copy import deepcopy
from datetime import UTC, datetime
from decimal import Decimal
from pathlib import Path

import pytest

from inflacao_scrapers.pao_de_acucar.catalog import validate_page
from inflacao_scrapers.pao_de_acucar.config import PaoDeAcucarSettings
from inflacao_scrapers.pao_de_acucar.offers import source_money
from inflacao_scrapers.pao_de_acucar.parsing import parse_products
from inflacao_scrapers.pao_de_acucar.source import validate_branch, validate_delivery
from inflacao_scrapers.scope import MonitoringTarget
from inflacao_scrapers.vtex.json import decode_response

FIXTURES = Path(__file__).parents[1] / "fixtures/stores"


def source(name):
    return decode_response((FIXTURES / f"pao-de-acucar-{name}.json").read_bytes())


def settings():
    configuration = PaoDeAcucarSettings(scraper_id="collector-sp-01")
    configuration.verify_context()
    return configuration


def parse(document, query):
    target = MonitoringTarget(
        id="test", name=query, search_queries=[query], required_terms=[[query]], excluded_terms=[]
    )
    return parse_products(
        document,
        settings(),
        "https://api.vendas.gpa.digital/pa/search/category-page",
        datetime(2026, 11, 1, 12, tzinfo=UTC),
        target,
    )


def test_packaged_identity_and_decimal_prices():
    page = parse(source("rice"), "arroz")
    assert not page.issues
    observation = page.observations[0]
    assert observation.product.source_product_id == "4485308"
    assert observation.offers[0].amount == "5.29"
    assert observation.product.package.quantity == "500"
    assert observation.context.postal_code == "04115090"
    assert observation.context.channel == "delivery"
    assert observation.product.gtin is None


def test_discount_keeps_reference_and_unknown_conditions():
    page = parse(source("coffee"), "pilão")
    offer = page.observations[0].offers[0]
    assert offer.amount == "23.99" and offer.reference_amount == "33.99"
    assert offer.conditions.membership_required is None
    assert offer.conditions.payment_method == "unknown"
    assert '"appExclusive": false' in offer.conditions.description


def test_missing_price_remains_unknown_and_stock_is_independent():
    document = source("rice")
    document["products"] = [document["products"][0]]
    document["products"][0].update(price=None, stock=False)
    observation = parse(document, "arroz").observations[0]
    assert observation.offers == [] and observation.availability == "out_of_stock"


def test_context_mapping_is_independent_of_default_store():
    validate_delivery(source("delivery"))
    validate_branch(source("branch")["content"])
    document = source("delivery")
    document["deliveryTypes"] = [
        entry for entry in document["deliveryTypes"] if entry["storeid"] != 461
    ]
    with pytest.raises(ValueError, match="missing or ambiguous"):
        validate_delivery(document)


def test_ambiguous_mapping_and_changed_branch_stop_collection():
    document = source("delivery")
    selected = next(entry for entry in document["deliveryTypes"] if entry["storeid"] == 461)
    document["deliveryTypes"].append(deepcopy(selected))
    with pytest.raises(ValueError, match="ambiguous"):
        validate_delivery(document)
    branch = source("branch")["content"]
    branch["city"] = "Guarulhos"
    with pytest.raises(ValueError, match="address changed"):
        validate_branch(branch)


def test_pagination_repeats_and_changed_totals_are_errors():
    document = source("rice")
    seen = set()
    assert validate_page(document, 1, seen, None) == (False, 58, 0)
    with pytest.raises(ValueError, match="repeated"):
        validate_page(document, 1, seen, 58)
    with pytest.raises(ValueError, match="changed"):
        validate_page(document, 1, set(), 59)
    document["products"].pop()
    with pytest.raises(ValueError, match="short/oversized"):
        validate_page(document, 1, set(), None)


@pytest.mark.parametrize("value", [1.1, True, Decimal("1.001"), Decimal("NaN"), -1])
def test_inexact_or_invalid_money_rejected(value):
    with pytest.raises(ValueError):
        source_money(value)


def test_unverified_promotions_are_visible_issues():
    document = source("coffee")
    document["products"] = [document["products"][0]]
    document["products"][0]["productPromotions"][0]["promotionQuantityBuy"] = 3
    page = parse(document, "café")
    assert not page.observations and "Unsupported" in page.issues[0]


def test_weight_uses_advertised_kg_instead_of_estimated_portion_total():
    page = parse(source("products"), "melão")
    assert not page.issues and len(page.observations) == 1
    observation = page.observations[0]
    assert observation.offers[0].amount == "12.99"
    assert observation.offers[0].basis == "kg"
    assert observation.product.package is None
    assert observation.product.source_product_id == "0339087"


def test_weight_without_advertised_kg_is_rejected():
    page = parse(source("products"), "castanha")
    assert not page.observations
    assert "variable-weight" in page.issues[0]
