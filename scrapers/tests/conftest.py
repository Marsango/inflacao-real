from datetime import UTC, datetime
from pathlib import Path

import pytest

from inflacao_scrapers.muffato import parse_products
from inflacao_scrapers.muffato.config import MuffatoSettings

FIXTURES = Path(__file__).parent / "fixtures"
ROOT = Path(__file__).resolve().parents[2]


def pytest_addoption(parser):
    parser.addoption("--live", action="store_true", help="Call the retailers' public APIs")


def pytest_collection_modifyitems(config, items):
    if config.getoption("--live"):
        return
    marker = pytest.mark.skip(reason="Use --live to check the actual retailer API")
    for item in items:
        mark_live_test(item, marker)


def mark_live_test(item, marker):
    if "live" in item.keywords:
        item.add_marker(marker)


@pytest.fixture
def settings():
    return MuffatoSettings(
        scraper_id="muffato-curitiba-01",
        sales_channel="8",
        store_id="sales-channel:8",
        city="Curitiba",
        state="PR",
    )


@pytest.fixture
def source_bytes():
    return (FIXTURES / "muffato-products.json").read_bytes()


@pytest.fixture
def observation(settings, source_bytes):
    page = parse_products(
        source_bytes,
        settings=settings,
        selected_sku="307840",
        source_url=settings.shop_url
        + "/api/catalog_system/pub/products/search?fq=skuId:307840&sc=8",
        observed_at=datetime(2026, 10, 29, 11, tzinfo=UTC),
    )
    assert not page.issues, page.issues
    return page.observations[0]


@pytest.fixture
def weighted_observation(settings):
    page = parse_products(
        (FIXTURES / "muffato-weight-products.json").read_bytes(),
        settings=settings,
        selected_sku="186092",
        source_url=settings.shop_url + "/api/catalog_system/pub/products/search?sc=8&ft=banana",
        observed_at=datetime(2026, 10, 29, 19, 4, tzinfo=UTC),
    )
    assert len(page.observations) == 1
    assert all("payment-basis" in issue for issue in page.warnings)
    return page.observations[0]
