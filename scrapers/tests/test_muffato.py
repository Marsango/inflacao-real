import json
from datetime import UTC, datetime
from decimal import Decimal

import httpx
import pytest
from conftest import FIXTURES
from jsonschema import Draft202012Validator, FormatChecker, ValidationError

from inflacao_scrapers.muffato import (
    decode_response,
    fetch_products,
    money,
    package_from_name,
    parse_products,
)
from inflacao_scrapers.transport import Requester


def parse(raw, settings):
    return parse_products(
        raw,
        settings=settings,
        selected_sku="307840",
        source_url=settings.shop_url + "/api",
        observed_at=datetime.now(UTC),
    )


def mutate(source_bytes, change):
    data = json.loads(source_bytes)
    change(data[0]["items"][0]["sellers"][0]["commertialOffer"])
    return json.dumps(data).encode()


def shape_validator():
    return Draft202012Validator(
        json.loads((FIXTURES / "muffato-response-schema.json").read_text()),
        format_checker=FormatChecker(),
    )


@pytest.mark.parametrize(
    "filename",
    ["muffato-products.json", "muffato-weight-products.json", "muffato-package-products.json"],
)
def test_saved_response_shape(filename):
    shape_validator().validate(decode_response((FIXTURES / filename).read_bytes()))


@pytest.mark.parametrize(
    "change",
    [
        lambda observation: observation.pop("Price"),
        lambda observation: observation.update(Price="16.66"),
        lambda observation: observation.update(IsAvailable="true"),
        lambda observation: observation.update(PaymentOptions=[]),
    ],
)
def test_response_shape_detects_breaking_changes(source_bytes, change):
    with pytest.raises(ValidationError):
        shape_validator().validate(decode_response(mutate(source_bytes, change)))


def test_fixture_parses_without_network_and_preserves_conditions(observation):
    assert observation.product.source_product_id == "307840"
    assert observation.product.gtin == "7894904296309"
    assert observation.product.package.model_dump() == {"quantity": "300", "unit": "g", "count": 1}
    assert observation.offers[0].amount == "16.66"
    assert observation.offers[0].reference_amount == "19.60"
    assert "Guerrilha Seara" in observation.offers[0].conditions.description
    assert observation.offers[0].conditions.coupon_required is None
    assert observation.context.store_id == "sales-channel:8"
    assert observation.context.postal_code is None
    assert observation.context.channel == "online"


@pytest.mark.parametrize(
    ("value", "expected"),
    [
        ("R$ 1.234,56", "1234.56"),
        ("19,6", "19.60"),
        (Decimal("16.660"), "16.66"),
        (0, "0.00"),
    ],
)
def test_brazilian_exact_prices(value, expected):
    assert money(value) == expected


@pytest.mark.parametrize("value", [1.2, True, "NaN", "Infinity", "1,234.56", "1.234,567", -1])
def test_invalid_prices_are_not_coerced(value):
    with pytest.raises((ValueError, ArithmeticError)):
        money(value)


@pytest.mark.parametrize(
    ("name", "expected"),
    [
        ("Arroz 5kg", {"quantity": "5", "unit": "kg", "count": 1}),
        ("Leite 1 L", {"quantity": "1", "unit": "l", "count": 1}),
        ("Café 0,5kg", {"quantity": "0.5", "unit": "kg", "count": 1}),
        ("Óleo 900ml", {"quantity": "900", "unit": "ml", "count": 1}),
        ("Pacote 6x350ml", None),
        ("Bebida 6 latas 350ml", None),
        ("Leite 12 unidades 1L", None),
        ("Café 500g + 50g", None),
        ("Leve 500g Pague 400g", None),
        ("Sem tamanho", None),
    ],
)
def test_package_extraction_is_conservative(name, expected):
    result = package_from_name(name)
    assert package_document(result) == expected


def package_document(package):
    if package is None:
        return None
    return package.model_dump()


def test_unavailable_zero_placeholder_is_not_a_free_price(source_bytes, settings):
    raw = mutate(
        source_bytes,
        lambda observation: observation.update(IsAvailable=False, Price=0, ListPrice=0),
    )
    page = parse(raw, settings)
    assert not page.issues
    assert page.observations[0].availability == "out_of_stock"
    assert page.observations[0].offers == []


def test_missing_price_stays_unknown(source_bytes, settings):
    page = parse(mutate(source_bytes, lambda observation: observation.update(Price=None)), settings)
    assert not page.issues
    assert page.observations[0].offers == []


def test_conditional_payment_uses_full_total(source_bytes, settings):
    def change(offer):
        offer["PaymentOptions"] = {
            "installmentOptions": [
                {
                    "paymentName": "Visa",
                    "paymentGroupName": "creditCardPaymentGroup",
                    "installments": [{"count": 3, "value": 600, "total": 1800}],
                }
            ]
        }
        offer["Teasers"] = [{"name": "Clube", "conditions": {"minimumQuantity": 3}}]

    page = parse(mutate(source_bytes, change), settings)
    assert not page.issues
    alternative = page.observations[0].offers[1]
    assert alternative.amount == "18.00"
    assert alternative.conditions.installments == 3
    assert "minimumQuantity" in alternative.conditions.description


def test_bad_barcode_preserves_observation_with_unknown_gtin(source_bytes, settings):
    data = json.loads(source_bytes)
    data[0]["items"][0]["ean"] = "7894904296300"
    page = parse(json.dumps(data).encode(), settings)
    assert any("barcode:" in issue for issue in page.issues)
    assert len(page.observations) == 1
    assert page.observations[0].product.gtin is None
    assert parse(b"[]", settings).issues
    assert parse(source_bytes, settings).observations


def test_json_decode_never_passes_money_through_float():
    assert decode_response(b'{"price":123456789.12}')["price"] == Decimal("123456789.12")
    with pytest.raises(ValueError):
        decode_response(b'{"price":1,"price":2}')
    with pytest.raises(ValueError):
        decode_response(b'{"price":NaN}')


def test_fetch_pins_branch_and_bypasses_result_caching(settings, source_bytes):
    requests = []

    def handler(request):
        requests.append(request)
        return httpx.Response(206, content=source_bytes)

    with httpx.Client(transport=httpx.MockTransport(handler)) as client:
        requester = Requester(client, interval=0)
        first = fetch_products(requester, settings, sku="307840")
        second = fetch_products(requester, settings, sku="307840")
    assert requests[0].url.params["sc"] == "8"
    assert requests[0].url.params["fq"] == "skuId:307840"
    assert requests[0].url.params["_ir"] != requests[1].url.params["_ir"]
    assert "no-cache" in requests[0].headers["cache-control"]
    assert "authorization" not in requests[0].headers
    a = parse_products(first[0], settings=settings, source_url=first[1], observed_at=first[2])
    b = parse_products(second[0], settings=settings, source_url=second[1], observed_at=second[2])
    assert a.observations[0].observation_id != b.observations[0].observation_id
    assert a.observations[0].observed_at < b.observations[0].observed_at


def test_aged_response_is_not_a_fresh_measurement(settings, source_bytes):
    transport = httpx.MockTransport(
        lambda request: httpx.Response(200, content=source_bytes, headers={"Age": "30"})
    )
    with httpx.Client(transport=transport) as client, pytest.raises(ValueError, match="cached"):
        fetch_products(Requester(client), settings, sku="307840")


@pytest.mark.live
@pytest.mark.parametrize("sku", ["25978", "27848", "8059"])
def test_live_api_response_shape_and_parser(settings, sku):
    with httpx.Client(timeout=20, follow_redirects=False) as client:
        raw, url, timestamp = fetch_products(Requester(client), settings, sku=sku)
    shape_validator().validate(decode_response(raw))
    page = parse_products(
        raw, settings=settings, source_url=url, observed_at=timestamp, selected_sku=sku
    )
    assert not page.issues, page.issues
    assert len(page.observations) == 1
