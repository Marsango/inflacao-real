import pytest

from inflacao_scrapers.commands import create_client, create_requester
from inflacao_scrapers.nagumo.adapter import ADAPTER
from inflacao_scrapers.nagumo.config import NagumoSettings


@pytest.mark.live
@pytest.mark.parametrize("query,basis", [("leite", "package"), ("banana", "kg")])
def test_live_curuca_pickup_discovery(query, basis):
    settings = NagumoSettings(scraper_id="diagnostic-sp")
    with create_client(settings) as client:
        requester = create_requester(client, settings)
        prepared = ADAPTER.prepare(requester, settings)
        page = ADAPTER.discover(requester, prepared, query, 1)
    assert not page.issues and len(page.observations) == 1
    item = page.observations[0]
    assert item.context.store_id == "13" and item.context.postal_code == "08030040"
    assert item.context.city == "São Paulo" and item.context.channel == "pickup"
    assert item.offers[0].basis == basis
    assert all(offer.conditions.membership_required is True for offer in item.offers[1:])
