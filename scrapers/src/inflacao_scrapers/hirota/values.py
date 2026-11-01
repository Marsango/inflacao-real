"""Exact source values; unknown values and internal barcodes stay unknown."""

import re
from decimal import Decimal
from urllib.parse import quote

from ..models.product import validate_gtin


def source_money(value) -> str | None:
    if value is None:
        return None
    if isinstance(value, bool) or not isinstance(value, (str, int, Decimal)):
        raise ValueError("Invalid exact monetary value")
    number = Decimal(value)
    if not number.is_finite() or number < 0 or number != number.quantize(Decimal("0.01")):
        raise ValueError("Invalid monetary precision or amount")
    if number == 0:
        return None
    return format(number, ".2f")


def source_gtin(value) -> str | None:
    if not isinstance(value, str):
        return None
    try:
        validate_gtin(value)
    except ValueError:
        return None
    return value


def product_identity(product) -> str:
    if not isinstance(product, dict):
        raise ValueError("Malformed Hirota product")
    identity = product["produto_id"]
    if type(identity) is not int or identity <= 0:
        raise ValueError("Invalid public product identity")
    return str(identity)


def product_path(product) -> str:
    slug = product["link"]
    if not isinstance(slug, str) or not re.fullmatch(r"[\w°&]+(?:-[\w°&]+)*", slug):
        raise ValueError("Invalid public product link")
    return "/produto/" + product_identity(product) + "/" + quote(slug, safe="-")
