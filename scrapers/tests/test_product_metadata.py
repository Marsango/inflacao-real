"""Malformed source metadata must not discard independently valid prices."""

import json
from datetime import UTC, datetime

import pytest

from inflacao_scrapers.vtex.parsing import parse_products


def parse_changed_product(source_bytes, settings, **changes):
    products = json.loads(source_bytes)
    product = products[0]
    product["link"] = changes.pop("link", product["link"])
    product["items"][0].update(changes)
    return parse_products(
        json.dumps([product]).encode(),
        settings=settings,
        source_url=settings.shop_url + "/catalog",
        observed_at=datetime.now(UTC),
        selected_sku="307840",
    )


@pytest.mark.parametrize(
    "link, ending",
    [
        ("/macarrao-nº2/p", "/macarrao-n%C2%BA2/p"),
        ("/snack-2°-unidade/p", "/snack-2%C2%B0-unidade/p"),
        ("/pão integral/p", "/p%C3%A3o%20integral/p"),
        ("/snack-2%C2%B0/p?variant=1&seller=1", "/snack-2%C2%B0/p?variant=1&seller=1"),
    ],
)
def test_product_links_are_encoded_without_double_encoding(source_bytes, settings, link, ending):
    page = parse_changed_product(source_bytes, settings, link=link)
    assert len(page.observations) == 1
    assert page.observations[0].product_url == settings.shop_url + ending
    assert not page.issues


@pytest.mark.parametrize("link", ["https://user:password@example.com/p", "/broken\npath/p"])
def test_invalid_product_links_remain_rejected(source_bytes, settings, link):
    page = parse_changed_product(source_bytes, settings, link=link)
    assert not page.observations
    assert page.issues


@pytest.mark.parametrize("ean", ["77330530057", "7894904296300", "ABC", 12345, False])
def test_invalid_barcodes_preserve_prices_and_sku(source_bytes, settings, ean):
    original = parse_changed_product(source_bytes, settings).observations[0]
    page = parse_changed_product(source_bytes, settings, ean=ean)
    assert len(page.observations) == 1
    measurement = page.observations[0]
    assert measurement.product.gtin is None
    assert measurement.product.source_product_id == original.product.source_product_id
    assert measurement.offers == original.offers
    assert any("barcode:" in issue for issue in page.warnings)


@pytest.mark.parametrize("ean", [None, "", "077330530057"])
def test_absent_or_valid_barcodes_do_not_report_errors(source_bytes, settings, ean):
    page = parse_changed_product(source_bytes, settings, ean=ean)
    assert len(page.observations) == 1
    assert page.observations[0].product.gtin == (ean or None)
    assert not page.issues


@pytest.mark.parametrize("unit", ["UN", "un", " Un "])
def test_unit_spelling_preserves_package_prices(source_bytes, settings, unit):
    page = parse_changed_product(source_bytes, settings, measurementUnit=unit)
    assert not page.issues
    assert page.observations[0].product.package.quantity == "300"
    assert page.observations[0].offers[0].basis == "package"
    assert page.observations[0].offers[0].amount == "16.66"


@pytest.mark.parametrize("unit", [None, 1, False, [], "unknown"])
def test_malformed_or_unknown_units_remain_errors(source_bytes, settings, unit):
    page = parse_changed_product(source_bytes, settings, measurementUnit=unit)
    assert not page.observations
    assert page.issues
