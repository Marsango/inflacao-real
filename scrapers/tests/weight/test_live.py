import httpx
import pytest

from inflacao_scrapers.transport import Requester
from inflacao_scrapers.vtex.fetching import fetch_products
from inflacao_scrapers.vtex.parsing import parse_products


@pytest.mark.live
@pytest.mark.parametrize("sku", ["186092", "269830", "145168"])
def test_live_muffato_advertised_kilogram_prices(settings, sku):
    with httpx.Client(timeout=25, follow_redirects=False) as client:
        raw, url, timestamp = fetch_products(Requester(client), settings, sku=sku)
    page = parse_products(
        raw, settings=settings, source_url=url, observed_at=timestamp, selected_sku=sku
    )
    assert len(page.observations) == 1, page.issues
    assert all("payment-basis:" in issue for issue in page.issues), page.issues
    observation = page.observations[0]
    assert observation.offers, "Missing advertised weight price; live verification failed"
    assert observation.product.package is None
    assert all(offer.basis == "kg" for offer in observation.offers)
    assert observation.context.store_id == "sales-channel:8"
