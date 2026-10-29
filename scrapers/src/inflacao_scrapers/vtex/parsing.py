"""Pure parsing: saved responses in, validated measurements and issues out."""

from dataclasses import dataclass
from datetime import datetime
from decimal import InvalidOperation
from urllib.parse import urljoin
from uuid import uuid4

from ..adapters import ParsedPage
from ..models import Context, Observation, Product
from ..scope import MonitoringScope, MonitoringTarget
from .config import VtexSettings
from .json import decode_response
from .offers import offers_from_source
from .prices import decimal_value, package_from_name


@dataclass
class ParsingContext:
    settings: VtexSettings
    source_url: str
    observed_at: datetime
    selected_sku: str | None
    target: MonitoringTarget | MonitoringScope | None


def parse_products(
    raw: bytes,
    *,
    settings: VtexSettings,
    source_url: str,
    observed_at: datetime,
    selected_sku: str | None = None,
    target: MonitoringTarget | MonitoringScope | None = None,
) -> ParsedPage:
    products = decode_response(raw)
    if isinstance(products, dict):
        products = products.get("products")
    if not isinstance(products, list):
        raise ValueError("Expected a product array, possibly a blocked/changed source")
    result, seen = ParsedPage(), set()
    context = ParsingContext(settings, source_url, observed_at, selected_sku, target)
    for product in products:
        parse_product(product, context, result, seen)
    finish_page(products, context, result, seen)
    return result


def parse_product(product, context: ParsingContext, result: ParsedPage, seen: set) -> None:
    if not isinstance(product, dict) or not isinstance(product.get("items"), list):
        result.issues.append("Malformed product or missing SKU array")
        return
    for sku in product["items"]:
        parse_candidate(sku, product, context, result, seen)


def parse_candidate(sku, product: dict, context: ParsingContext, result: ParsedPage, seen: set):
    identity = None
    if isinstance(sku, dict):
        identity = sku.get("itemId")
    if context.selected_sku is not None and identity != context.selected_sku:
        return
    try:
        parse_matching_candidate(sku, product, context, result, seen, identity)
    except (ValueError, KeyError, TypeError, InvalidOperation) as error:
        result.issues.append(f"sku={identity}: {error}")


def parse_matching_candidate(sku, product, context, result, seen, identity) -> None:
    if not matches_target(sku, context.target):
        return
    validate_identity(identity, seen)
    validate_packaged_sku(sku)
    result.observations.append(build_observation(sku, product, context))


def matches_target(sku, target: MonitoringTarget | MonitoringScope | None) -> bool:
    if target is None:
        return True
    if not isinstance(sku, dict):
        raise ValueError("Malformed SKU")
    name = sku.get("nameComplete") or sku["name"]
    if not isinstance(name, str) or not name.strip():
        raise ValueError("Missing SKU name")
    return target.matches(name)


def validate_identity(identity, seen: set) -> None:
    if not isinstance(identity, str) or not identity or identity in seen:
        raise ValueError("Missing or duplicated SKU identity")
    seen.add(identity)


def validate_packaged_sku(sku: dict) -> None:
    if sku.get("isKit") or sku.get("measurementUnit") != "un":
        raise ValueError("Initial collector supports individually sold packaged goods only")
    if decimal_value(sku.get("unitMultiplier")) != 1:
        raise ValueError("Initial collector supports individually sold packaged goods only")


def seller_offer(sku: dict, settings: VtexSettings) -> dict:
    sellers = [seller for seller in sku["sellers"] if seller["sellerId"] == settings.seller_id]
    if len(sellers) != 1:
        raise ValueError("Configured seller absent or ambiguous")
    return sellers[0]["commertialOffer"]


def source_product(sku: dict, product: dict) -> Product:
    name = sku.get("nameComplete") or sku["name"]
    return Product(
        source_product_id=sku["itemId"],
        name=name,
        brand=product.get("brand") or None,
        gtin=sku.get("ean") or None,
        package=package_from_name(name),
    )


def source_context(settings: VtexSettings) -> Context:
    settings.require_region()
    return Context(
        seller_id=settings.seller_id,
        store_id=settings.store_id,
        postal_code=settings.postal_code,
        city=settings.city,
        state=settings.state,
        channel=settings.observation_channel(),
    )


def availability(source: dict) -> str:
    available = source.get("IsAvailable")
    if available is not None and type(available) is not bool:
        raise ValueError("Malformed availability")
    if available is None:
        return availability_from_quantity(source.get("AvailableQuantity"))
    return {True: "in_stock", False: "out_of_stock", None: "unknown"}[available]


def availability_from_quantity(quantity) -> str:
    if quantity is None:
        return "unknown"
    if type(quantity) is not int or quantity < 0:
        raise ValueError("Malformed available quantity")
    if quantity > 0:
        return "in_stock"
    return "out_of_stock"


def build_observation(sku: dict, product: dict, context: ParsingContext) -> Observation:
    source = seller_offer(sku, context.settings)
    return Observation(
        observation_id=uuid4(),
        observed_at=context.observed_at,
        collection_method="api",
        source_url=context.source_url,
        product_url=urljoin(context.settings.shop_url, product["link"]),
        product=source_product(sku, product),
        context=source_context(context.settings),
        availability=availability(source),
        offers=offers_from_source(source),
    )


def finish_page(products: list, context: ParsingContext, result: ParsedPage, seen: set) -> None:
    if context.selected_sku is not None and context.selected_sku not in seen:
        result.issues.append(f"sku={context.selected_sku}: absent from source response")
    if not products:
        result.issues.append("Empty source response")
