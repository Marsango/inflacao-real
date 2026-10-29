"""Core-owned, brand-independent discovery scope and deterministic name rules."""

import re
import unicodedata
from typing import Annotated, Literal

from pydantic import Field, model_validator

from .config import Settings
from .delivery.client import DeliveryTarget
from .models.base import WireModel
from .transport import Requester

Term = Annotated[str, Field(min_length=1, max_length=200)]
Alternatives = Annotated[list[Term], Field(min_length=1, max_length=100)]


def normalize(value: str) -> str:
    decomposed = unicodedata.normalize("NFD", value.lower())
    plain = "".join(
        character for character in decomposed if unicodedata.category(character) != "Mn"
    )
    separated = re.sub(r"[^a-z0-9]+", " ", plain)
    return separated.strip()


def contains_any(name: str, terms: list[str]) -> bool:
    return any(f" {normalize(term)} " in name for term in terms)


class MonitoringTarget(WireModel):
    id: str = Field(pattern=r"^[a-z0-9]+(?:-[a-z0-9]+)*$")
    name: Term
    search_queries: Alternatives
    required_terms: list[Alternatives] = Field(min_length=1, max_length=100)
    excluded_terms: list[Term] = Field(max_length=100)

    def matches(self, name: str) -> bool:
        normalized = f" {normalize(name)} "
        if contains_any(normalized, self.excluded_terms):
            return False
        return all(contains_any(normalized, alternatives) for alternatives in self.required_terms)

    @model_validator(mode="after")
    def meaningful_terms(self):
        terms = [term for alternatives in self.required_terms for term in alternatives]
        if any(not normalize(term) for term in terms + self.excluded_terms + self.search_queries):
            raise ValueError("Monitoring terms must contain letters or digits")
        return self


class MonitoringScope(WireModel):
    schema_version: Literal["1.0"]
    targets: list[MonitoringTarget] = Field(max_length=1000)

    @model_validator(mode="after")
    def unique_targets(self):
        identities = [target.id for target in self.targets]
        if len(set(identities)) != len(identities):
            raise ValueError("Duplicate monitoring target IDs")
        return self


def fetch_scope(requester: Requester, settings: Settings, token: str) -> MonitoringScope:
    destination = DeliveryTarget(settings.core_url, token, settings.scraper_id, settings.shop_id)
    destination.validate()
    response = requester.request(
        "GET",
        settings.core_url + "/api/v1/monitoring-targets",
        headers={"Authorization": f"Bearer {token}", "Cache-Control": "no-cache"},
    )
    if response.status_code != 200:
        raise ValueError(f"Monitoring scope HTTP {response.status_code}; collection not started")
    return MonitoringScope.model_validate_json(response.content)
