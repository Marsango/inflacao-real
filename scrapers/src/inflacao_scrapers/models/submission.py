"""Immutable submission identities and their wire representation."""

from typing import Literal, Self

from pydantic import UUID4, AwareDatetime, field_serializer, model_validator

from .base import WireModel
from .observation import Observation
from .validation import encode_document, utc_time, validate_wire


class Collector(WireModel):
    name: str
    version: str


class Submission(WireModel):
    schema_version: Literal["1.0"]
    submission_id: UUID4
    scraper_id: str
    shop_id: str
    run_id: UUID4
    created_at: AwareDatetime
    collector: Collector
    observations: list[Observation]

    @field_serializer("created_at", when_used="json")
    def utc_time(self, value: AwareDatetime) -> str:
        return utc_time(value)

    @model_validator(mode="after")
    def contract(self) -> Self:
        validate_wire(self.model_dump(mode="json"))
        identities = [observation.observation_id for observation in self.observations]
        if len(identities) != len(set(identities)):
            raise ValueError("Duplicate observation identities")
        return self

    def encode(self) -> bytes:
        document = self.model_dump(mode="json")
        validate_wire(document)
        return encode_document(document)
