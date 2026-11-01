"""Pure Sonda card parsing with explicit unknown fulfillment provenance."""

import re
from uuid import uuid4

from ..adapters import ParsedPage
from ..models import Context, Observation, Product
from ..packaging import package_from_name
from .html import class_nodes, select, suffix_nodes, text, unique_text
from .offers import CONTEXT_NOTE, offers, price_basis
from .pages import product_identity


def parse_cards(cards, settings, source_url, observed_at, target=None):
    result = ParsedPage(warnings=[CONTEXT_NOTE])
    for card in cards:
        parse_candidate(card, settings, source_url, observed_at, target, result)
    return result


def parse_candidate(card, settings, source_url, observed_at, target, result):
    try:
        append_candidate(card, settings, source_url, observed_at, target, result)
    except (ValueError, TypeError, ArithmeticError) as error:
        identity = " ".join(text(node) for node in class_nodes(card, "js-sku"))
        result.issues.append(f"shop=sonda sku={identity}: {error}")


def observation(card, name, settings, source_url, observed_at):
    basis = price_basis(card, name)
    available = availability(card)
    prices = offers(card, basis) if available != "out_of_stock" else []
    return Observation(
        observation_id=uuid4(),
        observed_at=observed_at,
        collection_method="html",
        source_url=source_url,
        product_url=settings.shop_url + product_path(card),
        product=product_details(card, name),
        context=Context(
            seller_id=None, store_id=None, postal_code=None, city=None, state=None, channel="online"
        ),
        availability=available,
        offers=prices,
    )


def product_path(card):
    links = select(card, itemprop="url")
    if len(links) != 1:
        raise ValueError("Missing or ambiguous Sonda product link")
    path = links[0].attributes.get("href", "")
    if not re.fullmatch(r"/delivery/produto/[\w%.-]+/[1-9]\d*", path):
        raise ValueError("Invalid Sonda product URL")
    return path


def availability(card):
    if suffix_nodes(card, "_lnkAddToCarrinho"):
        return "in_stock"
    if suffix_nodes(card, "_pnlIndisponivel"):
        return "out_of_stock"
    return "unknown"


def append_candidate(card, settings, source_url, observed_at, target, result):
    name = unique_text(select(card, itemprop="name"), "product name")
    if target is not None and not target.matches(name):
        return
    item = observation(card, name, settings, source_url, observed_at)
    result.observations.append(item)
    if item.availability == "in_stock" and not item.offers:
        result.warnings.append(f"shop=sonda sku={item.product.source_product_id}: price unknown")


def product_details(card, name):
    return Product(
        source_product_id=product_identity(card),
        name=name,
        brand=None,
        gtin=None,
        package=package_from_name(name),
    )
