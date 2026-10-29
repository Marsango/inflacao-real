"""Exact VTEX prices and conservative package extraction."""

import re
from decimal import Decimal

from ..models import Package


def decimal_value(value) -> Decimal:
    if isinstance(value, bool) or not isinstance(value, (str, int, Decimal)):
        raise ValueError("Expected an exact price/quantity, never binary floating point")
    if isinstance(value, str):
        value = normalize_decimal_text(value)
    number = Decimal(value)
    if not number.is_finite() or number < 0:
        raise ValueError("Expected a finite nonnegative decimal")
    return number


def normalize_decimal_text(value: str) -> str:
    value = value.removeprefix("R$")
    value = value.strip()
    if "," in value:
        return normalize_brazilian_decimal(value)
    if not re.fullmatch(r"\d+(?:\.\d+)?", value):
        raise ValueError("Invalid decimal format")
    return value


def normalize_brazilian_decimal(value: str) -> str:
    if not re.fullmatch(r"(?:\d{1,3}(?:\.\d{3})+|\d+),\d+", value):
        raise ValueError("Invalid Brazilian decimal format")
    value = value.replace(".", "")
    return value.replace(",", ".")


def money(value) -> str:
    number = decimal_value(value)
    if number > Decimal("999999999.99") or number != number.quantize(Decimal("0.01")):
        raise ValueError("Price is outside the contract range/precision")
    return format(number, ".2f")


def ambiguous_package(name: str) -> bool:
    patterns = (
        r"\b(?:leve|pague|pack|kit|com\s+\d+)|\d\s*[xX×+]\s*\d",
        r"\b\d+\s*(?:unidades?|latas?|garrafas?|sach[eê]s?|c[aá]psulas?)\b",
    )
    if any(re.search(pattern, name, re.I) for pattern in patterns):
        return True
    measures = re.findall(r"\d+(?:[.,]\d+)?\s*(?:kg|g|ml|l)\b", name, re.I)
    return len(measures) > 1


def canonical_quantity(number: Decimal) -> str:
    value = format(number, "f")
    if "." in value:
        value = value.rstrip("0")
        return value.rstrip(".")
    return value


def package_from_name(name: str) -> Package | None:
    if ambiguous_package(name):
        return None
    match = re.search(r"(?<![\d.,])(\d+(?:[.,]\d+)?)\s*(kg|g|ml|l)\s*$", name, re.I)
    if match is None:
        return None
    quantity = decimal_value(match[1].replace(",", "."))
    if quantity <= 0:
        raise ValueError("Nonpositive package quantity")
    return Package(quantity=canonical_quantity(quantity), unit=match[2].lower(), count=1)
