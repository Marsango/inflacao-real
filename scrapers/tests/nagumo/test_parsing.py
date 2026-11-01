from decimal import Decimal

import pytest

from inflacao_scrapers.models import Submission
from inflacao_scrapers.nagumo.config import NagumoSettings
from inflacao_scrapers.nagumo.values import source_money

from .conftest import FIXTURES, parse, source


def test_captured_rice_and_milk_keep_regular_and_membership_prices_separate():
    page = parse()
    assert not page.issues and len(page.observations) == 9
    rice, milk = page.observations[0], page.observations[2]
    assert [offer.amount for offer in rice.offers] == ["23.70", "20.90"]
    assert [offer.amount for offer in milk.offers] == ["5.39", "4.79"]
    assert rice.offers[1].reference_amount == "23.70"
    conditions = rice.offers[1].conditions
    assert conditions.membership_required is True and conditions.membership_program == "Meu Nagumo"
    assert conditions.payment_method == "unknown" and conditions.coupon_required is None
    assert conditions.minimum_quantity is None and "ativação" in conditions.description
    assert rice.offers[0].conditions.membership_required is None
    assert rice.product.package.quantity == "5" and rice.product.package.unit == "kg"
    assert milk.product.package.quantity == "1" and milk.product.package.unit == "l"


def test_captured_ordinary_products_and_provenance_remain_exact():
    page = parse()
    assert [item.offers[0].amount for item in page.observations[1:5]] == [
        "8.95",
        "5.39",
        "25.90",
        "2.95",
    ]
    detergent = page.observations[4]
    assert detergent.product.package.quantity == "500" and detergent.product.package.unit == "ml"
    assert detergent.product_url.endswith("-359504.html")
    assert detergent.context.store_id == "13" and detergent.context.postal_code == "08030040"
    assert detergent.context.city == "São Paulo" and detergent.context.state == "SP"
    assert detergent.context.channel == "pickup" and detergent.context.seller_id is None
    assert all(item.product.gtin is None for item in page.observations)


def test_captured_weight_prices_use_advertised_kg_never_rounded_portions():
    bananas = parse().observations[5:]
    assert bananas[0].offers[0].amount == "9.98"
    assert [offer.amount for offer in bananas[1].offers] == ["6.98", "2.98"]
    assert all(item.product.package is None for item in bananas)
    assert all(offer.basis == "kg" for item in bananas for offer in item.offers)
    assert "peso final" in bananas[0].offers[0].conditions.description


def test_real_unavailable_listing_preserves_identity_and_has_no_offer():
    page = parse(source("rice-last"))
    assert not page.issues and len(page.observations) == 1
    item = page.observations[0]
    assert item.product.source_product_id == "252724"
    assert item.availability == "out_of_stock" and not item.offers


@pytest.mark.parametrize(
    "sales", [None, {"value": 0, "decimalPrice": "0.00", "formatted": "R$ 0,00", "currency": "BRL"}]
)
def test_synthetic_missing_and_zero_prices_stay_unknown(sales):
    document = source()
    document["productsSearchResult"][1]["price"]["sales"] = sales
    page = parse(document)
    assert not page.issues and page.warnings
    assert page.observations[1].offers == []


def test_synthetic_unknown_availability_cannot_claim_prices_or_stock():
    document = source()
    document["productsSearchResult"][0]["available"] = None
    item = parse(document).observations[0]
    assert item.availability == "unknown" and not item.offers


def test_known_member_price_survives_unknown_regular_price_without_invented_reference():
    document = source()
    document["productsSearchResult"][0]["price"]["sales"] = None
    page = parse(document)
    assert not page.issues
    offers = page.observations[0].offers
    assert len(offers) == 1 and offers[0].amount == "20.90"
    assert offers[0].reference_amount is None
    assert offers[0].conditions.membership_required is True


@pytest.mark.parametrize(
    "change", [{"weighable": "true"}, {"productMeasureValue": "litre"}, {"weighable": None}]
)
def test_synthetic_unsupported_sale_units_are_issues(change):
    document = source()
    product = document["productsSearchResult"][5]
    product.update(change)
    if change == {"weighable": None}:
        product["productMeasureValue"] = "weight"
    assert parse(document).issues


@pytest.mark.parametrize(
    "field,value", [("currency", "USD"), ("decimalPrice", "23.71"), ("formatted", "R$ 1,00")]
)
def test_synthetic_disagreeing_price_representations_are_issues(field, value):
    document = source()
    document["productsSearchResult"][0]["price"]["sales"][field] = value
    page = parse(document)
    assert page.issues and len(page.observations) == 8


@pytest.mark.parametrize(
    "change",
    [
        {"flagType": "NGM_26_M"},
        {"flagType": "pix"},
        {"valueFlag": "20.91"},
        {"valueFlag": "30.00", "valueFlagType": "R$ 30,00"},
    ],
)
def test_synthetic_unverified_or_inconsistent_promotions_fail_visibly(change):
    document = source()
    document["productsSearchResult"][0]["flagtypes"][0].update(change)
    page = parse(document)
    assert page.issues and all(
        item.product.source_product_id != "293303" for item in page.observations
    )


def test_synthetic_weight_total_and_average_weight_never_determine_the_price():
    document = source()
    banana = document["productsSearchResult"][5]
    banana["averageWeight"] = "999,999"
    banana["price"]["quantityValue"] = "0.01"
    assert parse(document).observations[5].offers[0].amount == "9.98"
    banana["price"]["sales"] = None
    assert not parse(document).observations[5].offers


@pytest.mark.parametrize("value", [0.1, True, "NaN", "Infinity", "-1", "3.001"])
def test_invalid_source_money_never_becomes_an_observation(value):
    with pytest.raises((ValueError, ArithmeticError)):
        source_money(value)
    assert source_money(Decimal("23.70")) == "23.70"


@pytest.mark.parametrize(
    "value",
    ["https://evil.example/293303.html", "https://www.nagumo.com.br/categoria/other-1.html"],
)
def test_synthetic_untrusted_or_mismatched_product_links_are_rejected(value):
    document = source()
    document["productsSearchResult"][0]["productShowFullUrl"] = value
    assert parse(document).issues


def test_unprepared_context_cannot_produce_observations():
    with pytest.raises(ValueError, match="verified"):
        parse(settings=NagumoSettings(scraper_id="test"))


@pytest.mark.parametrize(
    "change", [{"store_id": "26"}, {"postal_code": "01310200"}, {"city": "Santo André"}]
)
def test_only_the_verified_pickup_configuration_is_supported(change):
    with pytest.raises(ValueError):
        NagumoSettings(scraper_id="test", **change)


def test_saved_submission_validates_against_ingestion_v1():
    submission = Submission.model_validate_json((FIXTURES / "nagumo-submission.json").read_bytes())
    assert submission.shop_id == "nagumo" and len(submission.observations) == 9
    assert all(item.context.store_id == "13" for item in submission.observations)
