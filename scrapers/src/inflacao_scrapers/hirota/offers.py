"""Preserve advertised kg prices and promotion conditions, without dividing portions."""

import json
from decimal import Decimal

from ..models import Conditions, Offer
from .values import source_money


def sale_basis(product) -> str:
    if product.get("unidade_sigla") == "UN" and product.get("possui_unidade_diferente") is False:
        return "package"
    if product.get("unidade_sigla") == "KG" and product.get("exibe_preco_original") is True:
        return "kg"
    raise ValueError("Unsupported Hirota sale unit/portion format")


def product_offers(product, basis):
    if product.get("disponivel") is not True:
        return []
    promotion = product.get("oferta")
    if product.get("em_oferta") is True:
        return promotional_offer(product, promotion, basis)
    if product.get("em_oferta") is not False or promotion is not None:
        raise ValueError("Ambiguous Hirota promotion state")
    key = "preco_original" if basis == "kg" else "preco"
    return make_offer(product.get(key), None, basis, None, product)


def promotional_offer(product, promotion, basis):
    if not isinstance(promotion, dict):
        raise ValueError("Missing Hirota promotion details")
    if (
        promotion.get("tipo_oferta_id") != 1
        or promotion.get("categoria") != "G"
        or promotion.get("faixas_precos")
        or promotion.get("familia")
    ):
        raise ValueError("Unsupported Hirota promotion; conditions require verification")
    if promotion.get("quantidade_pagar") != 1 or promotion.get("quantidade_oferta") != 1:
        raise ValueError("Unsupported Hirota quantity promotion")
    amount, reference = promotional_amounts(product, promotion, basis)
    return make_offer(amount, reference, basis, promotion, product)


def promotional_amounts(product, promotion, basis):
    if basis != "kg":
        return promotion.get("preco_oferta"), promotion.get("preco_antigo")
    if promotion.get("exibe_preco_original") is not True:
        raise ValueError("Promotion has no advertised kg price")
    return promotion.get("preco_original"), product.get("preco_original")


def make_offer(value, reference, basis, promotion, product):
    amount = source_money(value)
    if amount is None:
        return []
    return [
        Offer(
            amount=amount,
            currency="BRL",
            basis=basis,
            reference_amount=source_money(reference),
            conditions=offer_conditions(basis, promotion, product),
        )
    ]


def condition_description(basis, promotion, product):
    description = "Condições de pagamento, clube e cupom não informadas."
    if basis == "kg":
        description += " Preço anunciado por kg; porção aproximada, total sujeito ao peso final."
    if promotion is not None:
        description += " Oferta: " + json.dumps(promotion, ensure_ascii=False, default=str)
    limits = {key: product.get(key) for key in ("quantidade_minima", "quantidade_maxima", "tags")}
    return (
        description + " Limites do anúncio: " + json.dumps(limits, ensure_ascii=False, default=str)
    )


def offer_conditions(basis, promotion, product):
    return Conditions(
        payment_method="unknown",
        installments=None,
        membership_required=None,
        membership_program=None,
        coupon_required=None,
        minimum_quantity=minimum_quantity(product, promotion),
        description=condition_description(basis, promotion, product),
    )


def minimum_quantity(product, promotion):
    values = [product.get("quantidade_minima")]
    if promotion is not None:
        values.append(promotion.get("quantidade_minima"))
    if None in values:
        return None
    quantities = [Decimal(str(value)) for value in values]
    if any(not quantity.is_finite() or quantity < 1 for quantity in quantities):
        raise ValueError("Invalid promotion minimum quantity")
    quantity = max(quantities)
    if quantity == 1:
        return None
    return format(quantity, "f")
