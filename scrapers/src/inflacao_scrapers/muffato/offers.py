"""Preserve promotions and distinct payment terms without price inference."""

import json
from decimal import Decimal

from ..models import Conditions, Offer
from .prices import money

PAYMENT_METHODS = {
    "creditCardPaymentGroup": "credit_card",
    "debitCardPaymentGroup": "debit_card",
    "instantPaymentPaymentGroup": "pix",
    "bankInvoicePaymentGroup": "boleto",
}


def promotion_entry(source: dict, key: str) -> str | None:
    value = source.get(key)
    if value:
        return f"{key}: {json.dumps(value, ensure_ascii=False, default=str)}"
    return None


def promotions(source: dict) -> str | None:
    keys = ("DiscountHighLight", "Teasers", "PromotionTeasers", "BuyTogether")
    descriptions = [promotion_entry(source, key) for key in keys]
    return "; ".join(filter(None, descriptions)) or None


def unknown_conditions(description: str | None) -> dict:
    return dict(
        payment_method="unknown",
        installments=None,
        membership_required=None,
        membership_program=None,
        coupon_required=None,
        minimum_quantity=None,
        description=description,
    )


def known_amount(source: dict) -> str | None:
    if source.get("Price") is None:
        return None
    amount = money(source["Price"])
    if amount != "0.00":
        return amount
    if source.get("IsAvailable") is False:
        return None
    raise ValueError("Unexplained zero price; verify the source")


def base_offer(source: dict, amount: str, conditions: dict) -> Offer:
    reference = None
    if source.get("ListPrice") is not None:
        reference = money(source["ListPrice"])
    return Offer(
        amount=amount,
        currency="BRL",
        basis="package",
        reference_amount=reference,
        conditions=Conditions(**conditions),
    )


def offers_from_source(source: dict) -> list[Offer]:
    amount = known_amount(source)
    if amount is None:
        return []
    conditions = unknown_conditions(promotions(source))
    result = [base_offer(source, amount, conditions)]
    terms = payment_terms(source)
    result.extend(payment_offer(key, names, conditions) for key, names in terms.items())
    return list({offer.model_dump_json(): offer for offer in result}.values())


def payment_terms(source: dict) -> dict:
    terms = {}
    options = source.get("PaymentOptions") or {}
    for option in options.get("installmentOptions", []):
        record_installments(terms, option)
    return terms


def record_installments(terms: dict, option: dict) -> None:
    for installment in option.get("installments", []):
        key = installment_terms(installment, option.get("paymentGroupName"))
        names = terms.setdefault(key, [])
        names.append(option.get("paymentName") or option.get("paymentGroupName") or "unknown")


def installment_terms(installment: dict, group: str | None) -> tuple:
    total, count = installment.get("total"), installment.get("count")
    if type(total) is not int:
        raise ValueError("Missing/noninteger installment total in minor units")
    if type(count) is not int or count < 1:
        raise ValueError("Invalid installment count")
    method = PAYMENT_METHODS.get(group, "other")
    installments, restriction = None, None
    if method == "credit_card":
        installments = count
    if method == "other":
        restriction = group
    return method, money(Decimal(total) / 100), installments, restriction


def payment_offer(key: tuple, names: list[str], conditions: dict) -> Offer:
    method, amount, count, _ = key
    detail = "Payment options: " + ", ".join(dict.fromkeys(names))
    description = "; ".join(filter(None, [conditions["description"], detail]))
    payment_conditions = Conditions(
        **{
            **conditions,
            "payment_method": method,
            "installments": count,
            "description": description,
        }
    )
    return Offer(
        amount=amount,
        currency="BRL",
        basis="package",
        reference_amount=None,
        conditions=payment_conditions,
    )
