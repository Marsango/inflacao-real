"""One measurement with source and location provenance."""

from typing import Literal, Self

from pydantic import UUID4, AwareDatetime, field_serializer, model_validator

from .base import WireModel
from .offer import Offer
from .product import Product
from .validation import utc_time, validate_url, validate_wire


class Context(WireModel):
    seller_id: str | None
    store_id: str | None
    postal_code: str | None
    city: str | None
    state: str | None
    channel: Literal["delivery", "pickup", "online", "unknown"]


class Observation(WireModel):
    observation_id: UUID4
    observed_at: AwareDatetime
    collection_method: Literal["api", "html", "browser"]
    source_url: str
    product_url: str
    product: Product
    context: Context
    availability: Literal["in_stock", "out_of_stock", "unknown"]
    offers: list[Offer]

    @field_serializer("observed_at", when_used="json")
    def utc_time(self, value: AwareDatetime) -> str:
        return utc_time(value)

    @model_validator(mode="after")
    def contract(self) -> Self:
        validate_url(self.source_url)
        validate_url(self.product_url)
        validate_wire(self.model_dump(mode="json"), "observation")
        return self

    def summary(self) -> dict:
        return {
            **self.product.describe(),
            "availability": self.availability,
            "prices": [offer.amount for offer in self.offers],
        }
