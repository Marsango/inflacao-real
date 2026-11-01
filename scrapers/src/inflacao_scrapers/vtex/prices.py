"""Exact VTEX prices and conservative package extraction."""

import re
from decimal import Decimal

from ..models.product import validate_gtin
from ..packaging import package_from_name as package_from_name


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


def sale_basis(sku: dict, settings) -> str:
    if sku.get("isKit") is not False:
        raise ValueError("sale-format: kits or missing kit status are unsupported")
    multiplier = decimal_value(sku.get("unitMultiplier"))
    if multiplier <= 0 or multiplier > maximum_multiplier(settings):
        raise ValueError("sale-format: unsupported unit multiplier")
    unit = measurement_unit(sku)
    if unit == "un" and multiplier == 1:
        return "package"
    if unit == "kg" and settings.supports_kilogram_prices():
        return "kg"
    raise ValueError(f"sale-format: unverified unit/multiplier {unit!r}/{multiplier}")


def maximum_multiplier(settings) -> Decimal:
    if settings.shop_id == "festval" and settings.supports_kilogram_prices():
        return Decimal(10)
    if settings.shop_id == "angeloni" and settings.supports_kilogram_prices():
        return Decimal(5)
    if settings.shop_id == "mambo" and settings.supports_kilogram_prices():
        return Decimal(3)
    return Decimal(1)


def source_gtin(sku: dict, basis: str, issues: list[str]) -> str | None:
    value = sku.get("ean")
    if value is None or value == "":
        return None
    if basis == "kg" and isinstance(value, str) and re.fullmatch(r"[0-9]{1,7}", value):
        return None  # Verified retailer internal codes, not GTINs; retain source SKU.
    if not isinstance(value, str):
        issues.append(f"barcode: omitted non-string source EAN {str(value)[:80]!r}")
        return None
    try:
        validate_gtin(value)
    except ValueError as error:
        issues.append(f"barcode: omitted invalid GTIN {value[:80]!r} ({error})")
        return None
    return value


def weight_description(sku: dict, basis: str) -> str | None:
    if basis != "kg":
        return None
    detail = sku.get("complementName")
    return "; ".join(filter(None, ["Preço anunciado por kg; total depende do peso final", detail]))


def measurement_unit(sku: dict) -> str:
    unit = sku.get("measurementUnit")
    if not isinstance(unit, str):
        raise ValueError("sale-format: missing or malformed measurement unit")
    return unit.strip().lower()
