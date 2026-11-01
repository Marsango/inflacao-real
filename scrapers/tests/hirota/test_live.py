"""Explicit read-only checks of the actual browser/API path; never ingest prices."""

from decimal import Decimal

import pytest

from inflacao_scrapers.commands import create_client, create_requester
from inflacao_scrapers.hirota.adapter import ADAPTER
from inflacao_scrapers.hirota.config import HirotaSettings


@pytest.mark.live
@pytest.mark.parametrize("query,basis", [("leite", "package"), ("tomate", "kg")])
def test_live_nazare_pickup_discovery(query, basis):
    settings = HirotaSettings(scraper_id="diagnostic-sp")
    with create_client(settings) as client:
        requester = create_requester(client, settings)
        settings = ADAPTER.prepare(requester, settings)
        result = ADAPTER.discover(requester, settings, query, 1)
    assert not result.issues and len(result.observations) == 1
    observation = result.observations[0]
    assert observation.context.channel == "pickup"
    assert observation.context.postal_code == "04263100"
    assert observation.context.city == "São Paulo" and observation.context.state == "SP"
    assert observation.offers[0].basis == basis
    assert Decimal(observation.offers[0].amount) > 0
