"""Keep regular and membership offers separate; never divide portion totals."""

import json

from ..models import Conditions, Offer
from .values import displayed_money, regular_amount, source_money


def sale_basis(product):
    weighable, measure = product["weighable"], product.get("productMeasureValue")
    if weighable is True and measure in ("unity", "weight"):
        return "kg"
    if (weighable is None or weighable is False) and measure == "unity":
        return "package"
    raise ValueError("Unsupported Nagumo variable-weight marker")


def product_offers(product, basis):
    if product.get("available") is not True:
        return []
    price = product.get("price")
    if not isinstance(price, dict) or price.get("defaultPrice") or price.get("list"):
        raise ValueError("Unsupported Nagumo price structure")
    if product.get("promotionDiscount"):
        raise ValueError("Unsupported Nagumo promotion discount")
    amount = regular_amount(price.get("sales"))
    offers = [] if amount is None else [make_offer(amount, None, basis, product, None)]
    flags = product.get("flagtypes")
    if flags is None or flags == []:
        return offers
    offers.append(membership_offer(product, flags, amount, basis))
    return offers


def membership_offer(product, flags, reference, basis):
    if not isinstance(flags, list) or len(flags) != 1 or not isinstance(flags[0], dict):
        raise ValueError("Unsupported Nagumo multiple/malformed promotion flags")
    flag = flags[0]
    image = product.get("flagImage")
    if (
        flag.get("flagType") != "NGM_13_M"
        or not isinstance(image, str)
        or not image.endswith("/meunagumo.png")
    ):
        raise ValueError("Unsupported Nagumo promotion; membership conditions require verification")
    amount = source_money(flag.get("valueFlag"))
    if amount is None or amount != displayed_money(flag.get("valueFlagType")):
        raise ValueError("Nagumo promotion numeric and displayed prices disagree")
    return make_offer(amount, reference, basis, product, flag)


def make_offer(amount, reference, basis, product, flag):
    return Offer(
        amount=amount,
        reference_amount=reference,
        basis=basis,
        currency="BRL",
        conditions=Conditions(
            payment_method="unknown",
            installments=None,
            membership_required=True if flag is not None else None,
            membership_program="Meu Nagumo" if flag is not None else None,
            coupon_required=None,
            minimum_quantity=None,
            description=condition_description(product, basis, flag),
        ),
    )


def condition_description(product, basis, flag):
    description = "Pagamento, cupom e quantidade mínima não informados."
    if basis == "kg":
        description += " Preço anunciado por kg; total sujeito ao peso final, sem pacote fixo."
    if flag is not None:
        description += " Oferta Meu Nagumo; ativação e elegibilidade adicionais não verificadas."
    details = {
        key: product.get(key) for key in ("tags", "productObs", "averageWeight", "acceleratorRule")
    }
    details["flag"] = flag
    return description + " Anúncio: " + json.dumps(details, ensure_ascii=False, default=str)
