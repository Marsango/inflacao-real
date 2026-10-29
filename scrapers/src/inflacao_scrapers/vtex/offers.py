"""Preserve VTEX promotions and distinct payment terms without price inference."""

import json
from decimal import Decimal, InvalidOperation

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
    if source.get("IsAvailable") is False or source.get("AvailableQuantity") == 0:
        return None
    raise ValueError("Unexplained zero price; verify the source")


def reference_amount(source: dict, amount: str, issues: list[str]) -> str | None:
    if source.get("ListPrice") is None:
        return None
    try:
        return validated_reference(source["ListPrice"], amount)
    except (ValueError, InvalidOperation) as error:
        issues.append(f"reference-price: omitted invalid reference ({error})")
        return None


def validated_reference(value, amount: str) -> str:
    reference = money(value)
    if Decimal(reference) < Decimal(amount):
        raise ValueError("reference is lower than selling price")
    return reference


def base_offer(source: dict, amount: str, conditions: dict, basis: str, issues: list[str]) -> Offer:
    return Offer(
        amount=amount,
        currency="BRL",
        basis=basis,
        reference_amount=reference_amount(source, amount, issues),
        conditions=Conditions(**conditions),
    )


def offers_from_source(
    source: dict,
    basis: str = "package",
    description: str | None = None,
    issues: list[str] | None = None,
) -> list[Offer]:
    if issues is None:
        issues = []
    amount = known_amount(source)
    if amount is None:
        return []
    detail = "; ".join(filter(None, [promotions(source), description])) or None
    conditions = unknown_conditions(detail)
    result = [base_offer(source, amount, conditions, basis, issues)]
    result.extend(conditional_offers(source, basis, conditions, issues))
    return list({offer.model_dump_json(): offer for offer in result}.values())


def conditional_offers(
    source: dict, basis: str, conditions: dict, issues: list[str]
) -> list[Offer]:
    entries = payment_entries(source, issues)
    if basis == "kg" and entries:
        issues.append("payment-basis: portion totals omitted; no verified per-kg payment price")
    if basis == "kg":
        return []
    terms = {}
    for option in entries:
        collect_payment_terms(terms, option, issues)
    return validated_payment_offers(terms, conditions, issues)


def payment_entries(source: dict, issues: list[str]) -> list:
    options = source.get("PaymentOptions") or {}
    if not isinstance(options, dict) or not isinstance(options.get("installmentOptions", []), list):
        issues.append("payment-offer: malformed payment options omitted")
        return []
    return options.get("installmentOptions", [])


def collect_payment_terms(terms: dict, option: dict, issues: list[str]) -> None:
    if not isinstance(option, dict) or not isinstance(option.get("installments", []), list):
        issues.append("payment-offer: malformed installment options omitted")
        return
    for installment in option.get("installments", []):
        collect_installment(terms, option, installment, issues)


def collect_installment(terms: dict, option: dict, installment: dict, issues: list[str]) -> None:
    if not isinstance(installment, dict):
        issues.append("payment-offer: malformed installment omitted")
        return
    try:
        key = installment_terms(installment, option.get("paymentGroupName"))
        names = terms.setdefault(key, [])
        names.append(option.get("paymentName") or option.get("paymentGroupName") or "unknown")
    except (ValueError, InvalidOperation) as error:
        issues.append(f"payment-offer: omitted invalid terms ({error})")


def validated_payment_offers(terms: dict, conditions: dict, issues: list[str]) -> list[Offer]:
    result = []
    for key, names in terms.items():
        append_payment_offer(result, key, names, conditions, issues)
    return result


def append_payment_offer(result, key, names, conditions, issues) -> None:
    try:
        result.append(payment_offer(key, names, conditions))
    except ValueError as error:
        issues.append(f"payment-offer: omitted invalid offer ({error})")


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
