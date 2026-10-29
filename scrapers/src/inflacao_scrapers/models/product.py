"""Source product identity and package facts."""

from typing import Literal, Self

from pydantic import model_validator

from .base import WireModel


def validate_gtin(value: str) -> None:
    if not value.isascii() or not value.isdigit() or len(value) not in (8, 12, 13, 14):
        raise ValueError("Invalid GTIN shape")
    weights = (3, 1)
    total = sum(
        int(digit) * weights[position % 2] for position, digit in enumerate(reversed(value[:-1]))
    )
    if (-total) % 10 != int(value[-1]):
        raise ValueError("Invalid GTIN check digit")


class Package(WireModel):
    quantity: str
    unit: Literal["g", "kg", "ml", "l", "unit"]
    count: int


class Product(WireModel):
    source_product_id: str
    name: str
    brand: str | None
    gtin: str | None
    package: Package | None

    @model_validator(mode="after")
    def valid_gtin(self) -> Self:
        if self.gtin is not None:
            validate_gtin(self.gtin)
        return self

    def describe(self) -> dict:
        package = None
        if self.package is not None:
            package = self.package.model_dump()
        return {"sku": self.source_product_id, "name": self.name, "package": package}
