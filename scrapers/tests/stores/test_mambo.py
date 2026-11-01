import sys
from datetime import UTC, datetime
from pathlib import Path

import httpx
import pytest

from inflacao_scrapers import cli
from inflacao_scrapers.config import load_configurations
from inflacao_scrapers.vtex.parsing import parse_products
from inflacao_scrapers.vtex.regional import prepare_region

from .conftest import FIXTURES, load_sample
from .test_pipeline import install_mock_runtime, monitoring_response, submissions


def parse_mambo(raw=None, prepared=True):
    sample = load_sample("mambo")
    settings = sample.prepared_settings() if prepared else sample.settings
    return parse_products(
        sample.source if raw is None else raw,
        settings=settings,
        source_url=settings.shop_url + "/api/catalog_system/pub/products/search?ft=arroz",
        observed_at=datetime(2026, 11, 1, 1, tzinfo=UTC),
    )


def test_captured_mambo_packaged_prices_preserve_delivery_context_and_unknown_conditions():
    page = parse_mambo()
    assert not page.issues and not page.warnings
    assert len(page.observations) == 7
    rice, unavailable, milk, beans, coffee, detergent, unavailable_detergent = page.observations
    assert [item.offers[0].amount for item in [rice, milk, beans, coffee, detergent]] == [
        "34.98", "9.29", "11.98", "26.99", "2.98"
    ]
    assert detergent.offers[0].reference_amount == "3.59"
    assert coffee.offers[0].reference_amount == "49.98"
    assert rice.context.postal_code == "01310200" and rice.context.city == "São Paulo"
    assert rice.context.state == "SP" and rice.context.channel == "delivery"
    assert rice.context.seller_id == "1" and "mambodelivery215" in rice.context.store_id
    assert rice.offers[0].conditions.membership_required is None
    assert unavailable.availability == unavailable_detergent.availability == "out_of_stock"
    assert unavailable.offers == unavailable_detergent.offers == []
    assert rice.product.package.quantity == "5"
    assert detergent.product.package.unit == "ml"


def test_verified_weight_prices_keep_kilogram_basis_and_omit_estimated_payment_totals():
    page = parse_mambo((FIXTURES / "mambo-weight-candidates.json").read_bytes())
    assert not page.issues
    assert len(page.observations) == 5
    expected = {"1260": "9.99", "7425": "79.98", "11549": "19.90",
                "9274": "14.98", "287726": "12.98"}
    for observation in page.observations:
        assert observation.product.package is None
        assert observation.product.gtin is None
        assert len(observation.offers) == 1
        assert observation.offers[0].basis == "kg"
        assert observation.offers[0].amount == expected[observation.product.source_product_id]
        assert "peso final" in observation.offers[0].conditions.description
    assert len(page.warnings) == 5
    assert all("payment-basis:" in warning for warning in page.warnings)


def test_unprepared_mambo_delivery_context_cannot_produce_observations():
    page = parse_mambo(prepared=False)
    assert not page.observations
    assert all("must be verified" in issue for issue in page.issues)


def test_sao_paulo_example_preserves_distinct_retailer_contexts_and_spools():
    path = Path(__file__).parents[2] / "sao-paulo.example.toml"
    settings = load_configurations(path)
    assert [item.adapter for item in settings] == [
        "mambo", "hirota", "pao-de-acucar", "sonda", "nagumo"
    ]
    assert settings[0].spool_path == "data/mambo/spool.sqlite3"
    assert settings[1].spool_path == "data/hirota/spool.sqlite3"
    assert len({item.spool_path for item in settings}) == len(settings)
    assert settings[4].store_id == "13" and settings[4].postal_code == "08030040"
    assert settings[4].city == "São Paulo" and settings[4].state == "SP"
    assert settings[3].store_id is None and settings[3].city is None


def mambo_handler(sample, requests, wrong_region):
    def handler(request):
        requests.append(request)
        if request.url.host == "127.0.0.1":
            return monitoring_response() if request.method == "GET" else httpx.Response(202)
        assert "authorization" not in request.headers
        if request.url.path.endswith("/regions"):
            assert request.url.params["postalCode"] == "01310200"
            return httpx.Response(200, content=b"[]" if wrong_region else sample.regions)
        assert "vtex_segment=" in request.headers["cookie"]
        assert request.url.params["regionId"] == sample.prepared_settings().region_id
        return httpx.Response(200, content=sample.source)
    return handler


def execute_mambo(tmp_path, monkeypatch, wrong_region=False):
    path = tmp_path / "config.toml"
    path.write_text('[[scrapers]]\nadapter="mambo"\nscraper_id="shared"\n')
    requests = []
    install_mock_runtime(monkeypatch, mambo_handler(load_sample("mambo"), requests, wrong_region))
    monkeypatch.setenv("INGESTION_TOKEN", "secret")
    monkeypatch.setattr(sys, "argv", ["scraper", "--config", str(path), "collect"])
    return cli.main(), requests


def test_mambo_collects_core_scope_and_delivers_without_creating_a_spool(tmp_path, monkeypatch):
    status, requests = execute_mambo(tmp_path, monkeypatch)
    documents = submissions(requests)
    assert status == 0
    assert len(documents) == 1
    assert documents[0]["shop_id"] == documents[0]["collector"]["name"] == "mambo"
    assert len(documents[0]["observations"]) == 2
    assert not (tmp_path / "data").exists()


def test_wrong_mambo_regional_seller_stops_before_prices_or_delivery(tmp_path, monkeypatch):
    status, requests = execute_mambo(tmp_path, monkeypatch, wrong_region=True)
    assert status == 1
    assert not submissions(requests)
    assert not any("products/search" in request.url.path for request in requests)


@pytest.mark.live
def test_live_mambo_packaged_catalog():
    from inflacao_scrapers.commands import create_client, create_requester

    sample = load_sample("mambo")
    with create_client(sample.settings) as client:
        requester = create_requester(client, sample.settings)
        settings = prepare_region(requester, sample.settings)
        page = sample.adapter.discover(requester, settings, "arroz branco", 10)
    assert not page.issues, page.issues
    assert page.observations
    assert any(item.offers and item.availability == "in_stock" for item in page.observations)
    assert all(item.context.postal_code == "01310200" for item in page.observations)


@pytest.mark.live
@pytest.mark.parametrize(
    "query,identity", [("banana nanica", "1260"), ("patinho", "7425"),
                       ("pao frances", "11549"), ("melao", "9274")],
)
def test_live_mambo_kilogram_catalog(query, identity):
    from inflacao_scrapers.commands import create_client, create_requester

    sample = load_sample("mambo")
    with create_client(sample.settings) as client:
        requester = create_requester(client, sample.settings)
        settings = prepare_region(requester, sample.settings)
        page = sample.adapter.discover(requester, settings, query, 10)
    assert not page.issues, page.issues
    weighted = [item for item in page.observations if item.product.source_product_id == identity]
    assert len(weighted) == 1
    assert all(len(item.offers) == 1 and item.offers[0].basis == "kg" for item in weighted)
    assert all(item.context.postal_code == "01310200" for item in weighted)
