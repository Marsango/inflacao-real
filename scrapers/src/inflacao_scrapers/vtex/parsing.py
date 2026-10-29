"""Pure parsing: saved responses in, validated measurements and issues out."""

from dataclasses import dataclass
from datetime import datetime
from decimal import InvalidOperation
from urllib.parse import urljoin
from uuid import uuid4

from ..adapters import ParsedPage
from ..models import Context, Observation, Product
from ..packaging import resolve_package
from ..scope import MonitoringScope, MonitoringTarget
from .config import VtexSettings
from .json import decode_response
from .offers import offers_from_source
from .prices import sale_basis, source_gtin, weight_description


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
        result.issues.append(f"shop={context.settings.shop_id} sku={identity}: {error}")


def parse_matching_candidate(sku, product, context, result, seen, identity) -> None:
    if not matches_target(sku, context.target):
        return
    validate_identity(identity, seen)
    basis = sale_basis(sku, context.settings)
    issues = []
    result.observations.append(build_observation(sku, product, context, basis, issues))
    result.issues.extend(
        f"shop={context.settings.shop_id} sku={identity}: {issue}" for issue in issues
    )


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


def seller_offer(sku: dict, settings: VtexSettings) -> dict:
    sellers = [seller for seller in sku["sellers"] if seller["sellerId"] == settings.seller_id]
    if len(sellers) != 1:
        raise ValueError("Configured seller absent or ambiguous")
    return sellers[0]["commertialOffer"]


def source_product(sku: dict, product: dict, basis: str, settings, issues: list[str]) -> Product:
    name = sku.get("nameComplete") or sku["name"]
    return Product(
        source_product_id=sku["itemId"],
        name=name,
        brand=product.get("brand") or None,
        gtin=source_gtin(sku, basis),
        package=source_package(sku, basis, settings, issues),
    )


def source_package(sku: dict, basis: str, settings, issues: list[str]):
    try:
        return resolve_package(sku, basis, settings)
    except (ValueError, InvalidOperation) as error:
        issues.append(f"package-size: {error}")
        return None


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


def build_observation(sku, product, context: ParsingContext, basis: str, issues) -> Observation:
    source = seller_offer(sku, context.settings)
    return Observation(
        observation_id=uuid4(),
        observed_at=context.observed_at,
        collection_method="api",
        source_url=context.source_url,
        product_url=urljoin(context.settings.shop_url, product["link"]),
        product=source_product(sku, product, basis, context.settings, issues),
        context=source_context(context.settings),
        availability=availability(source),
        offers=offers_from_source(source, basis, weight_description(sku, basis), issues),
    )


def finish_page(products: list, context: ParsingContext, result: ParsedPage, seen: set) -> None:
    if context.selected_sku is not None and context.selected_sku not in seen:
        result.issues.append(f"sku={context.selected_sku}: absent from source response")
    if not products:
        result.issues.append("Empty source response")
