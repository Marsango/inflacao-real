"""Super Muffato adapter: public fetching and fixture-parsing entry points."""

from .fetching import fetch_products
from .json import decode_response
from .offers import offers_from_source, promotions
from .parsing import ParsedPage, parse_products
from .prices import decimal_value, money, package_from_name

__all__ = [
    "ParsedPage",
    "decode_response",
    "decimal_value",
    "fetch_products",
    "money",
    "offers_from_source",
    "package_from_name",
    "parse_products",
    "promotions",
]
