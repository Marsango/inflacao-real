"""Prices and the conditions that make them meaningful."""

from decimal import Decimal
from typing import Literal, Self

from pydantic import model_validator

from .base import Money, WireModel


class Conditions(WireModel):
    payment_method: Literal[
        "any", "pix", "cash", "debit_card", "credit_card", "boleto", "other", "unknown"
    ]
    installments: int | None
    membership_required: bool | None
    membership_program: str | None
    coupon_required: bool | None
    minimum_quantity: str | None
    description: str | None


class Offer(WireModel):
    amount: Money
    currency: Literal["BRL"]
    basis: Literal["package", "kg", "l", "unit"]
    reference_amount: Money | None
    conditions: Conditions

    @model_validator(mode="after")
    def reference_not_lower(self) -> Self:
        if self.reference_amount is not None and Decimal(self.reference_amount) < Decimal(
            self.amount
        ):
            raise ValueError("Reference price is lower than the selling price")
        return self
