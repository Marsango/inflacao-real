"""Keep source restrictions; never divide an estimated portion price."""

import json
from decimal import Decimal

from ..models import Conditions, Offer


def source_money(value):
    if value is None:
        return None
    if isinstance(value, bool) or not isinstance(value, (int, Decimal)):
        raise ValueError("Invalid exact GPA price")
    amount = Decimal(value)
    if not amount.is_finite() or amount < 0 or amount != amount.quantize(Decimal("0.01")):
        raise ValueError("Invalid GPA money precision/range")
    return format(amount, ".2f")


def price_details(product):
    if product.get("variableWeight") is False:
        return product, "package"
    details = product.get("variableWeightPrice")
    if product.get("variableWeight") is True and isinstance(details, dict):
        return details, "kg"
    raise ValueError("Unsupported GPA variable-weight portion or missing sale basis")


def product_offers(product):
    details, basis = price_details(product)
    amount = source_money(details.get("price"))
    if amount is None:
        return []
    validate_promotions(details)
    return [
        Offer(
            amount=amount,
            currency="BRL",
            basis=basis,
            reference_amount=source_money(details.get("priceFrom")),
            conditions=source_conditions(product, details, basis),
        )
    ]


def validate_promotions(details):
    promotions = details.get("productPromotions", [])
    if not isinstance(promotions, list):
        raise ValueError("Malformed GPA promotion list")
    if len(promotions) > 1:
        raise ValueError("Multiple GPA promotions require separate verified offer mapping")
    for promotion in promotions:
        validate_promotion(promotion, details)
    if details.get("productPromotion") not in (None, *promotions):
        raise ValueError("GPA selected promotion missing from promotion list")


def validate_promotion(promotion, details):
    if not isinstance(promotion, dict):
        raise ValueError("Malformed GPA promotion")
    if any(
        promotion.get(key)
        for key in (
            "promotionQuantityBuy",
            "promotionQuantityPayFor",
            "paymentFormName",
            "subscriptionPrice",
            "appExclusive",
            "isHeterogeneousPromotion",
        )
    ):
        raise ValueError("Unsupported GPA quantity/payment/application promotion")
    if source_money(promotion.get("unitPrice")) != source_money(details.get("price")):
        raise ValueError("GPA promotion price differs from displayed price")


def source_conditions(product, details, basis):
    description = "Catálogo storeId=461, customerPlus=true; pagamento/clube/cupom não verificados."
    if basis == "kg":
        description += " Preço por kg; total depende da pesagem final."
    restrictions = {key: details.get(key) for key in ("productPromotions", "productPromotion")}
    restrictions["priceType"] = product.get("priceType")
    description += " Fonte: " + json.dumps(restrictions, ensure_ascii=False, default=str)
    return Conditions(
        payment_method="unknown",
        installments=None,
        membership_required=None,
        membership_program=None,
        coupon_required=None,
        minimum_quantity=None,
        description=description,
    )
