"""Bounded SKU-name fallback; no unverified structured content fields."""

import re
from decimal import Decimal

from ..models import Package
from ..models.validation import validate_wire

UNITS = {
    "g": ("g", Decimal(1)),
    "kg": ("g", Decimal(1000)),
    "ml": ("ml", Decimal(1)),
    "l": ("ml", Decimal(1000)),
}
MEASUREMENT = re.compile(r"(?<![\d.,])(-?\d+(?:[.,]\d+)?)\s*(kg|g|ml|l)\b", re.I)
AMBIGUOUS = re.compile(
    r"\b(?:leve|pague|pack|kit|aprox\w*|estimad\w*|porç\w*|porc\w*|drenad\w*|bruto)\b"
    r"|\+|\d\s*[x×–-]\s*\d|\bpor\s+\d|R\$\s*/|\d\s*(?:a|até)\s*\d",
    re.I,
)
COMPONENTS = re.compile(
    r"\b\d+\s*(?:un(?:idades?)?|latas?|garrafas?|sach[eê]s?|c[aá]psulas?)\b", re.I
)
PROTEIN_CLAIM = re.compile(r"\b\d+(?:[.,]\d+)?\s*g\s*(?:de\s+)?(?:prote[ií]nas?|proteins?)\b", re.I)
MENTOS_LINE = re.compile(r"(\bmentos\s+)3l\b", re.I)
NON_CONTENT_MEASUREMENTS = re.compile(r"\bfraldas?\b|\bsacos?\b.*\blixo\b", re.I)


class AmbiguousPackage(ValueError):
    """A single SKU name describes multiple distinct product sizes."""


def canonical_quantity(number: Decimal) -> str:
    value = format(number, "f")
    if "." in value:
        value = value.rstrip("0")
        return value.rstrip(".")
    return value


def package_value(quantity: str, unit: str) -> Package:
    number = Decimal(quantity.replace(",", "."))
    document = {"quantity": canonical_quantity(number), "unit": unit.lower(), "count": 1}
    validate_wire(document, "package")
    return Package(**document)


def equivalent_contents(package: Package) -> tuple:
    unit, multiplier = UNITS.get(package.unit, (package.unit, Decimal(1)))
    return unit, Decimal(package.quantity) * multiplier, package.count


def reconcile(packages: list[Package]) -> Package | None:
    if not packages:
        return None
    declarations = {equivalent_contents(package) for package in packages}
    if len(declarations) != 1:
        raise ValueError("conflicting SKU content declarations")
    return packages[0]


def name_declarations(name: str) -> list[Package]:
    if NON_CONTENT_MEASUREMENTS.search(name) or AMBIGUOUS.search(name) or COMPONENTS.search(name):
        return []
    contents = PROTEIN_CLAIM.sub("", name)
    contents = MENTOS_LINE.sub(r"\1", contents)
    packages = [package_value(match[1], match[2]) for match in MEASUREMENT.finditer(contents)]
    if len({equivalent_contents(package) for package in packages}) > 1:
        raise AmbiguousPackage("ambiguous package: multiple distinct weights in SKU name")
    return packages


def package_from_name(name: str) -> Package | None:
    return reconcile(name_declarations(name))


def sku_names(sku: dict) -> list[str]:
    return list(dict.fromkeys(name for name in (sku.get("name"), sku.get("nameComplete")) if name))


def egg_package(names: list[str], settings) -> Package | None:
    if not settings.supports_egg_counts() or not all(re.match(r"^ovos\b", n, re.I) for n in names):
        return None
    declarations = [egg_declaration(name) for name in names]
    return reconcile([package for package in declarations if package is not None])


def egg_declaration(name: str) -> Package | None:
    if AMBIGUOUS.search(name) or MEASUREMENT.search(name):
        return None
    match = re.search(r"\b(\d+)\s*(?:un|unidades?)\s*$", name, re.I)
    if match is None:
        return None
    return package_value(match[1], "unit")


def resolve_package(sku: dict, basis: str, settings) -> Package | None:
    if basis != "package":
        return None
    names = sku_names(sku)
    if any(NON_CONTENT_MEASUREMENTS.search(name) for name in names):
        return None
    if any(AMBIGUOUS.search(name) for name in names):
        return None
    packages = [package for name in names for package in name_declarations(name)]
    if packages:
        return reconcile(packages)
    return egg_package(names, settings)
