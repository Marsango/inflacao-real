"""Opt-in, read-only diagnostics; never ingest live data."""

import pytest

from inflacao_scrapers.commands import create_client, create_requester
from inflacao_scrapers.pao_de_acucar.adapter import ADAPTER
from inflacao_scrapers.pao_de_acucar.config import PaoDeAcucarSettings


@pytest.mark.live
@pytest.mark.parametrize("query,basis", [("arroz", "package"), ("melão", "kg")])
def test_live_delivery_discovery(query, basis):
    settings = PaoDeAcucarSettings(scraper_id="diagnostic-sp")
    with create_client(settings) as client:
        requester = create_requester(client, settings)
        prepared = ADAPTER.prepare(requester, settings)
        page = ADAPTER.discover(requester, prepared, query, 1)
    assert not page.issues and len(page.observations) == 1
    observation = page.observations[0]
    assert observation.context.store_id == "461"
    assert observation.context.postal_code == "04115090"
    assert observation.offers[0].basis == basis
