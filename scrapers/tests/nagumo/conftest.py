from datetime import UTC, datetime

from stores.conftest import FIXTURES

from inflacao_scrapers.nagumo.config import NagumoSettings
from inflacao_scrapers.nagumo.parsing import parse_products
from inflacao_scrapers.vtex.json import decode_response


def source(name="samples"):
    return decode_response((FIXTURES / f"nagumo-{name}.json").read_bytes())


def prepared_settings(**changes):
    settings = NagumoSettings(scraper_id="collector-sp-01", **changes)
    settings.confirm_context()
    return settings


def parse(document=None, settings=None):
    return parse_products(
        source() if document is None else document,
        prepared_settings() if settings is None else settings,
        "https://www.nagumo.com.br/on/demandware.store/Sites-Nagumo-Site/pt_BR/Search-UpdateGrid?q=arroz&start=0&sz=24",
        datetime(2026, 11, 1, 11, 14, tzinfo=UTC),
    )
