"""Use visible BRL amounts; never trust template metadata or analytics floats."""

import re
from decimal import Decimal

from ..models import Conditions, Offer
from ..scope import normalize
from .html import class_nodes, suffix_nodes, text, unique_text

CONTEXT_NOTE = (
    "Catálogo público online Sonda. Loja, CEP e condições de retirada não selecionados; "
    "uniformidade regional de preços não verificada. Estoque sujeito à data da entrega. "
    "Condições de pagamento, clube e cupom não informadas."
)


def money(value):
    if value is None:
        return None
    if not re.fullmatch(r"(?:\d{1,3}(?:\.\d{3})+|\d+),\d{2}", value):
        raise ValueError("Invalid Sonda visible BRL price")
    amount = Decimal(value.replace(".", "").replace(",", "."))
    if amount == 0:
        return None
    return format(amount, ".2f")


def price_basis(card, name):
    category = unique_text(class_nodes(card, "js-titulo"), "category")
    fresh = ("acougue", "hortifruti", "peixaria", "padaria", "frios e laticinios queijos")
    if any(word in normalize(category) for word in fresh):
        raise ValueError("Sonda fresh/variable-weight price basis requires verification")
    verify_dairy(category, name)
    if re.search(r"(?:/\s*kg|por\s+(?:kg|quilo)|peso\s+vari[aá]vel|aprox\w*)", name, re.I):
        raise ValueError("Sonda kg label alone does not establish a per-kg price")
    return "package"


def offers(card, basis):
    badges = class_nodes(card, "product--badges")
    description = " ".join(text(node) for node in badges).strip()
    if description:
        raise ValueError(f"Unsupported Sonda promotion/badge: {description}")
    value = unique_text(suffix_nodes(card, "_lblValorPor"), "selling price", False)
    amount = money(value)
    if amount is None:
        return []
    reference = money(unique_text(suffix_nodes(card, "_lblValorDe"), "reference price", False))
    return [
        Offer(
            amount=amount,
            currency="BRL",
            basis=basis,
            reference_amount=reference,
            conditions=conditions(card),
        )
    ]


def conditions(card):
    discounts = class_nodes(card, "product--discount")
    labels = [unique_text([node], "discount") for node in discounts]
    if any(not re.fullmatch(r"\d+% OFF", label) for label in labels):
        raise ValueError("Unsupported Sonda discount conditions")
    return Conditions(
        payment_method="unknown",
        installments=None,
        membership_required=None,
        membership_program=None,
        coupon_required=None,
        minimum_quantity=None,
        description=CONTEXT_NOTE + " " + " ".join(labels),
    )


def verify_dairy(category, name):
    if "frios e laticinios" not in normalize(category):
        return
    fixed = ("leite ", "iogurte ", "manteiga ", "margarina ", "requeijao ", "bebida lactea ")
    if not normalize(name).startswith(fixed):
        raise ValueError("Sonda deli/cheese weight basis requires verification")
