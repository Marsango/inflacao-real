"""Source identities, exact money and trusted public product links."""

import re
from decimal import Decimal
from urllib.parse import urlsplit


def source_money(value) -> str | None:
    if value is None:
        return None
    if isinstance(value, bool) or not isinstance(value, (str, int, Decimal)):
        raise ValueError("Invalid exact Nagumo monetary value")
    number = Decimal(value)
    if not number.is_finite() or number < 0 or number != number.quantize(Decimal("0.01")):
        raise ValueError("Invalid Nagumo monetary precision or amount")
    if number == 0:
        return None
    return format(number, ".2f")


def displayed_money(value):
    if not isinstance(value, str) or not re.fullmatch(r"R\$\s*\d{1,3}(?:\.\d{3})*,\d{2}", value):
        raise ValueError("Malformed Nagumo displayed BRL amount")
    number = value.removeprefix("R$")
    number = number.strip()
    number = number.replace(".", "")
    return source_money(number.replace(",", "."))


def regular_amount(record):
    if record is None:
        return None
    if not isinstance(record, dict) or record.get("currency") != "BRL":
        raise ValueError("Missing Nagumo BRL price record")
    amount = source_money(record.get("decimalPrice"))
    if amount != source_money(record.get("value")) or amount != displayed_money(
        record.get("formatted")
    ):
        raise ValueError("Nagumo exact and displayed prices disagree")
    return amount


def product_identity(product):
    if not isinstance(product, dict):
        raise ValueError("Malformed Nagumo product")
    identity = product.get("id")
    if not isinstance(identity, str) or not re.fullmatch(r"[1-9][0-9]*", identity):
        raise ValueError("Invalid Nagumo public product identity")
    return identity


def product_url(product):
    value = product.get("productShowFullUrl")
    if not isinstance(value, str):
        raise ValueError("Missing Nagumo public product URL")
    parts = urlsplit(value)
    if (
        parts.scheme != "https"
        or parts.netloc != "www.nagumo.com.br"
        or parts.query
        or parts.fragment
    ):
        raise ValueError("Untrusted Nagumo public product URL")
    if not parts.path.startswith("/categoria/") or not parts.path.endswith(
        f"-{product_identity(product)}.html"
    ):
        raise ValueError("Nagumo product URL does not match the source identity")
    return value
