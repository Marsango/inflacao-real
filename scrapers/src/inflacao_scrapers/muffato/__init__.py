"""Super Muffato adapter: public fetching and fixture-parsing entry points."""

from ..vtex.fetching import fetch_products
from ..vtex.json import decode_response
from ..vtex.offers import offers_from_source, promotions
from ..vtex.parsing import ParsedPage, parse_products
from ..vtex.prices import decimal_value, money, package_from_name

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
