import json
from datetime import UTC, datetime
from pathlib import Path

import pytest

from inflacao_scrapers.vtex.json import decode_response
from inflacao_scrapers.vtex.parsing import parse_products

FIXTURES = Path(__file__).parents[1] / "fixtures"


@pytest.fixture
def weighted_products():
    return decode_response((FIXTURES / "muffato-weight-products.json").read_bytes())


@pytest.fixture
def packaged_products():
    return decode_response((FIXTURES / "muffato-package-products.json").read_bytes())


def parse(products, settings, **selection):
    return parse_products(
        json.dumps(products, default=str, ensure_ascii=False).encode(),
        settings=settings,
        source_url=settings.shop_url + "/api/catalog_system/pub/products/search?sc=8",
        observed_at=datetime.now(UTC),
        **selection,
    )


def rename(product, name):
    product["items"][0].update(name=name, nameComplete=name)


def source_offer(product):
    return product["items"][0]["sellers"][0]["commertialOffer"]
