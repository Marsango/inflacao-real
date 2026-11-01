"""Ordinary paced HTTP; no customer cookies, authentication or search routes."""

from ..vtex.json import decode_response
from .config import API_BASE


def request_document(requester, path, *, parameters=None, body=None):
    method = "GET" if body is None else "POST"
    arguments = {"params": parameters, "headers": {"Cache-Control": "no-cache"}}
    if body is not None:
        arguments["json"] = body
    response = requester.request(method, API_BASE + path, **arguments)
    if response.status_code != 200:
        raise ValueError(f"Pão de Açúcar HTTP {response.status_code}; collection stopped")
    document = decode_response(response.content)
    if not isinstance(document, dict) or document.get("error"):
        raise ValueError("Pão de Açúcar malformed/failed response")
    return document, str(response.request.url)


def prepare_context(requester, settings):
    document, _ = request_document(
        requester, "delivery-v2/ecom/deliveryOptions", parameters={"zipCode": settings.postal_code}
    )
    validate_delivery(document)
    document, _ = request_document(requester, "stores/461/humanization")
    validate_branch(document.get("content"))
    prepared = settings.model_copy(deep=True)
    prepared.verify_context()
    return prepared


def validate_delivery(document):
    entries = document.get("deliveryTypes")
    if not isinstance(entries, list):
        raise ValueError("Missing Pão de Açúcar delivery mapping")
    matches = [
        entry for entry in entries if isinstance(entry, dict) and entry.get("storeid") == 461
    ]
    if len(matches) != 1:
        raise ValueError("Pão de Açúcar Ricardo Jafet mapping missing or ambiguous")
    entry = matches[0]
    if entry.get("typeid") != 842 or entry.get("erpCode") != "1717":
        raise ValueError("Pão de Açúcar delivery modality changed")
    if entry.get("name") != "ENTREGA EXPRESS":
        raise ValueError("Pão de Açúcar delivery modality unknown")
    validate_delivery_address(entry.get("address"))


def validate_delivery_address(address):
    if not isinstance(address, dict):
        raise ValueError("Missing GPA delivery branch address")
    city = address.get("city")
    if not isinstance(city, str) or city.strip() != "SAO PAULO" or address.get("state") != "SP":
        raise ValueError("Pão de Açúcar delivery branch outside verified São Paulo context")


def validate_branch(branch):
    expected = {"id": 461, "zip": "04115-090", "city": "SAO PAULO", "addressNumber": 299}
    if not isinstance(branch, dict) or any(
        branch.get(key) != value for key, value in expected.items()
    ):
        raise ValueError("Pão de Açúcar Ricardo Jafet address changed")
    if branch.get("name") != "1717 - Ricardo Jafet":
        raise ValueError("Pão de Açúcar branch identity changed")
