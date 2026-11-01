"""Select pickup in an isolated anonymous client, then fail on any context loss."""

from ..vtex.json import decode_response
from .config import ROUTE_PREFIX


def request_document(requester, settings, route, parameters=None):
    response = requester.request(
        "GET",
        settings.shop_url + ROUTE_PREFIX + route,
        params=parameters,
        headers={"Cache-Control": "no-cache", "Pragma": "no-cache"},
    )
    if response.status_code != 200:
        raise ValueError(f"Nagumo {route} HTTP {response.status_code}; collection stopped")
    document = decode_response(response.content)
    if not isinstance(document, dict) or document.get("action") != route:
        raise ValueError(f"Nagumo malformed {route} response; collection stopped")
    if document.get("error") or document.get("success") is False:
        raise ValueError(f"Nagumo failed {route} response; collection stopped")
    return document, str(response.request.url)


def prepare_context(requester, settings):
    prepared = settings.model_copy(deep=True)
    cart, _ = request_document(requester, prepared, "Cart-Get")
    if cart.get("numItems") != 0 or cart.get("items") != []:
        raise ValueError("Nagumo requires a fresh anonymous empty session")
    document, _ = request_document(requester, prepared, "Stores-AvailableStores")
    validate_branches(document, require_active=False)
    selection, _ = request_document(
        requester,
        prepared,
        "Stores-SelectStore",
        {"storeId": prepared.store_id, "method": "Retirada"},
    )
    if selection.get("success") is not True:
        raise ValueError("Nagumo pickup selection was not confirmed")
    verify_context(requester, prepared)
    prepared.confirm_context()
    return prepared


def verify_context(requester, settings) -> None:
    document, _ = request_document(requester, settings, "Stores-AvailableStores")
    validate_branches(document, require_active=True)


def validate_branches(document, *, require_active):
    branches = document.get("stores")
    if not isinstance(branches, list) or any(not isinstance(branch, dict) for branch in branches):
        raise ValueError("Nagumo branch list missing or malformed")
    matches = [branch for branch in branches if branch.get("storeId") == "13"]
    if len(matches) != 1:
        raise ValueError("Nagumo Vila Curuçá branch missing or ambiguous")
    validate_branch(matches[0])
    active = [branch for branch in branches if branch.get("active") is True]
    if require_active and (len(active) != 1 or active[0].get("storeId") != "13"):
        raise ValueError("Nagumo selected pickup context was lost; collection stopped")


def validate_branch(branch):
    expected = {
        "storeName": "013-CURUCA",
        "city": "SAO PAULO",
        "district": "VILA CURUCA",
        "address": "PRACA FRANCISCO PEREIRA, VILA CURUCA, SAO PAULO, SP",
        "operation": True,
    }
    if any(branch.get(key) != value for key, value in expected.items()):
        raise ValueError("Nagumo branch does not match the verified São Paulo pickup address")
    if branch.get("operation") is not True:
        raise ValueError("Nagumo Vila Curuçá branch is not operational")
