from copy import deepcopy
from datetime import UTC, datetime
from decimal import Decimal

import pytest
from stores.conftest import FIXTURES

from inflacao_scrapers.hirota.catalog import validate_page
from inflacao_scrapers.hirota.config import API_BASE, CATALOG_PREFIX, HirotaSettings
from inflacao_scrapers.hirota.parsing import parse_products
from inflacao_scrapers.hirota.session import validate_context, validate_pickup
from inflacao_scrapers.hirota.values import source_money
from inflacao_scrapers.vtex.json import decode_response


def source(name="products"):
    return decode_response((FIXTURES / f"hirota-{name}.json").read_bytes())


def prepared_settings():
    settings = HirotaSettings(scraper_id="collector-sp-01")
    settings.bind_session({"Authorization": "anonymous-test-session"})
    return settings


def parse(document=None, settings=None):
    return parse_products(
        source() if document is None else document,
        prepared_settings() if settings is None else settings,
        API_BASE + CATALOG_PREFIX + "produtos/mais_vendidos?page=1",
        datetime(2026, 11, 1, 3, tzinfo=UTC),
    )


def test_captured_packaged_and_weight_prices_match_rendered_nazare_pickup():
    page = parse()
    assert not page.issues
    assert len(page.observations) == 10
    milk = page.observations[1]
    assert milk.offers[0].amount == "4.88"
    assert milk.offers[0].reference_amount == "6.48"
    assert milk.offers[0].conditions.membership_required is None
    assert milk.product.gtin == "7898080640611"
    assert milk.context.channel == "pickup" and milk.context.postal_code == "04263100"
    assert milk.context.city == "São Paulo" and milk.context.state == "SP"
    assert milk.context.store_id == "org:171/filial:1/centro:1"
    weighted = page.observations[4:8]
    assert [item.offers[0].amount for item in weighted] == ["19.00", "13.00", "20.00", "7.98"]
    assert all(item.offers[0].basis == "kg" and item.product.package is None for item in weighted)
    assert all(item.product.gtin is None for item in weighted)
    assert page.observations[7].offers[0].reference_amount == "10.98"


def test_captured_rice_beans_and_cleaning_packages_are_exact():
    cereals, cleaning = parse(source("cereals")), parse(source("cleaning"))
    assert not cereals.issues and not cleaning.issues
    rice = next(item for item in cereals.observations if item.product.source_product_id == "498")
    beans = next(item for item in cereals.observations if item.product.source_product_id == "1859")
    detergent = next(
        item for item in cleaning.observations if item.product.source_product_id == "606"
    )
    assert rice.product.package.quantity == "5" and rice.product.package.unit == "kg"
    assert beans.product.package.quantity == "1" and beans.product.package.unit == "kg"
    assert detergent.product.package.quantity == "500" and detergent.product.package.unit == "ml"
    assert rice.product_url.endswith("/produto/498/arroz-camil-tipo-1-5kg")


@pytest.mark.parametrize("value", [None, "0.00"])
def test_synthetic_unknown_prices_are_never_zero(value):
    document = source()
    document["data"][0]["preco"] = value
    page = parse(document)
    assert not page.issues and page.warnings
    assert page.observations[0].offers == []


def test_synthetic_unavailable_and_unknown_availability_do_not_invent_offers():
    document = source()
    document["data"][0]["disponivel"] = False
    document["data"][1]["disponivel"] = None
    page = parse(document)
    assert page.observations[0].availability == "out_of_stock"
    assert page.observations[1].availability == "unknown"
    assert not page.observations[0].offers and not page.observations[1].offers


@pytest.mark.parametrize(
    "change",
    [
        {"unidade_sigla": "L"},
        {"exibe_preco_original": False},
        {"preco_original": "-1"},
        {"preco_original": "7.981"},
    ],
)
def test_synthetic_unsupported_weight_formats_cannot_become_portion_prices(change):
    document = source()
    document["data"] = [document["data"][4]]
    document["data"][0].update(change)
    page = parse(document)
    assert page.issues and not page.observations


def test_synthetic_unknown_weight_price_does_not_divide_rounded_portion():
    document = source()
    document["data"] = [document["data"][4]]
    document["data"][0]["preco_original"] = None
    page = parse(document)
    assert not page.issues
    assert page.observations[0].offers == []


@pytest.mark.parametrize(
    "change",
    [
        {"tipo_oferta_id": 2},
        {"categoria": "C"},
        {"quantidade_oferta": 3},
        {"familia": True},
        {"faixas_precos": [{"quantidade": 2, "preco": "1.00"}]},
    ],
)
def test_synthetic_unverified_promotion_shapes_are_reported(change):
    document = source()
    document["data"] = [document["data"][1]]
    document["data"][0]["oferta"].update(change)
    page = parse(document)
    assert page.issues and not page.observations


def test_synthetic_minimum_quantity_remains_conditional():
    document = source()
    document["data"] = [document["data"][1]]
    document["data"][0]["oferta"]["quantidade_minima"] = 3
    offer = parse(document).observations[0].offers[0]
    assert offer.conditions.minimum_quantity == "3"
    assert '"quantidade_minima": 3' in offer.conditions.description
    assert offer.conditions.payment_method == "unknown"


@pytest.mark.parametrize("value", [0.1, True, "NaN", "Infinity", "-1", "3.001"])
def test_invalid_exact_money_is_rejected(value):
    with pytest.raises((ValueError, ArithmeticError)):
        source_money(value)
    assert source_money(Decimal("7.98")) == "7.98"


def test_unprepared_context_cannot_parse_and_sessions_never_serialize():
    settings = HirotaSettings(scraper_id="test")
    with pytest.raises(ValueError, match="verified"):
        parse(settings=settings)
    settings.bind_session({"Authorization": "private-session"})
    assert "private-session" not in settings.model_dump_json()
    assert "private-session" not in repr(settings)


def test_captured_pickup_context_validates_and_ambiguous_mapping_fails():
    branch = source("context")["data"]
    validate_context(branch)
    validate_pickup([branch])
    with pytest.raises(ValueError, match="ambiguous"):
        validate_pickup([branch, branch])
    branch["endereco"]["cidade"] = "Curitiba"
    with pytest.raises(ValueError, match="São Paulo"):
        validate_context(branch)


@pytest.mark.parametrize("field,value", [("ativo", False), ("id", 4), ("endereco", {})])
def test_changed_or_missing_branch_stops_collection(field, value):
    branch = source("context")["data"]
    branch[field] = value
    with pytest.raises(ValueError):
        validate_context(branch)


def test_captured_pagination_counts_all_source_rows_and_rejects_repeat():
    document, seen = source(), set()
    assert validate_page(document, 1, seen, None) == (False, 40)
    repeated = deepcopy(document)
    repeated["paginator"]["page"] = 2
    with pytest.raises(ValueError, match="repeated"):
        validate_page(repeated, 2, seen, 40)


@pytest.mark.parametrize(
    "change",
    [
        {"page": 2},
        {"items_per_page": 0},
        {"total_pages": 3},
        {"total_items": -1},
        {"page": True},
    ],
)
def test_malformed_pagination_is_not_successful_exhaustion(change):
    document = source()
    document["paginator"].update(change)
    with pytest.raises(ValueError):
        validate_page(document, 1, set(), None)


def test_short_page_and_changing_total_remain_failures():
    document = source()
    document["data"].pop()
    with pytest.raises(ValueError, match="short"):
        validate_page(document, 1, set(), None)
    with pytest.raises(ValueError, match="changed"):
        validate_page(source(), 1, set(), 39)


def test_captured_litre_spelling_and_coffee_contents_remain_fixed_packages():
    milk = parse().observations[1]
    assert milk.product.package.quantity == "1" and milk.product.package.unit == "l"
    coffee = parse(source("coffee"))
    assert not coffee.issues
    selected = next(item for item in coffee.observations if item.product.source_product_id == "88")
    assert selected.offers[0].amount == "24.98"
    assert selected.product.package.quantity == "90" and selected.product.package.unit == "g"


def test_synthetic_listing_minimum_and_maximum_are_preserved_even_without_promotion():
    document = source()
    document["data"] = [document["data"][0]]
    document["data"][0].update(quantidade_minima="3", quantidade_maxima="6")
    offer = parse(document).observations[0].offers[0]
    assert offer.conditions.minimum_quantity == "3"
    assert '"quantidade_maxima": "6"' in offer.conditions.description


def test_saved_hirota_submission_remains_valid_ingestion_v1():
    from inflacao_scrapers.models import Submission

    submission = Submission.model_validate_json((FIXTURES / "hirota-submission.json").read_bytes())
    assert submission.shop_id == "hirota" and len(submission.observations) == 7
    assert all(item.context.channel == "pickup" for item in submission.observations)
    assert all(item.context.postal_code == "04263100" for item in submission.observations)
