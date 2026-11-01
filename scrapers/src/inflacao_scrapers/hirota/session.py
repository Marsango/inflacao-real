"""Resolve and validate public branch provenance for every run."""

from ..vtex.json import decode_response
from .browser import bootstrap_session
from .config import API_BASE, CATALOG_PREFIX


def request_document(requester, settings, path, parameters=None):
    response = requester.request(
        "GET",
        API_BASE + path,
        params=parameters,
        headers=settings.source_headers(),
    )
    if response.status_code != 200:
        raise ValueError(f"Hirota catalog HTTP {response.status_code}; collection stopped")
    document = decode_response(response.content)
    if not isinstance(document, dict) or document.get("success") is not True:
        raise ValueError("Hirota malformed/failed response; collection stopped")
    return document, str(response.request.url)


def prepare_context(requester, settings):
    prepared = settings.model_copy(deep=True)
    prepared.bind_session(bootstrap_session(settings))
    document, _ = request_document(requester, prepared, "org/171/loja/centros_distribuicoes/1")
    validate_context(document.get("data"))
    document, _ = request_document(
        requester,
        prepared,
        "org/171/filial/1/loja/centros_distribuicoes/retiradas",
    )
    validate_pickup(document.get("data"))
    return prepared


def validate_context(branch) -> None:
    if not isinstance(branch, dict) or branch.get("id") != 1 or branch.get("ativo") is not True:
        raise ValueError("Hirota Nazaré branch missing, changed or inactive")
    if type(branch["id"]) is not int:
        raise ValueError("Hirota branch identity must be an integer")
    address = branch.get("endereco", {})
    expected = {
        "cep": "04263100",
        "logradouro": "Avenida Nazaré",
        "numero": "1299",
        "cidade": "São Paulo",
        "estado": "São Paulo",
    }
    if not isinstance(address, dict) or any(
        address.get(key) != value for key, value in expected.items()
    ):
        raise ValueError("Hirota branch does not match the verified São Paulo pickup address")


def validate_pickup(branches) -> None:
    if not isinstance(branches, list):
        raise ValueError("Hirota pickup branch list missing")
    matches = [branch for branch in branches if isinstance(branch, dict) and branch.get("id") == 1]
    if len(matches) != 1:
        raise ValueError("Hirota pickup branch missing or ambiguous")
    validate_context(matches[0])


def departments(requester, settings):
    document, _ = request_document(
        requester,
        settings,
        CATALOG_PREFIX + "classificacoes_mercadologicas/departamentos/arvore",
    )
    entries = document.get("data")
    if not isinstance(entries, list) or not entries:
        raise ValueError("Hirota department tree missing")
    if any(not isinstance(entry, dict) for entry in entries):
        raise ValueError("Malformed Hirota department")
    identities = [entry.get("classificacao_mercadologica_id") for entry in entries]
    if any(type(identity) is not int or identity <= 0 for identity in identities):
        raise ValueError("Hirota department identity malformed")
    if len(set(identities)) != len(identities):
        raise ValueError("Hirota repeated department identity")
    return identities
