import copy

import pytest

from inflacao_scrapers.packaging.resolution import equivalent_contents

from .conftest import parse, rename


def test_observed_nonterminal_duplicate_and_count_names(packaged_products, settings):
    page = parse(packaged_products, settings)
    assert not page.issues, page.issues
    packages = [observation.product.package for observation in page.observations]
    assert packages[0].model_dump() == {"quantity": "12", "unit": "unit", "count": 1}
    assert packages[1].quantity == "500"
    assert packages[2].quantity == "10"
    assert packages[3] is None  # Total box contents versus per-capsule size is unverified.


@pytest.mark.parametrize(
    "name,quantity,unit",
    [("Café 500 g Tradicional", "500", "g"), ("Leite 0,5 L Integral", "0.5", "l")],
)
def test_bounded_nonterminal_fallback(packaged_products, settings, name, quantity, unit):
    rename(packaged_products[0], name)
    package = parse(packaged_products[:1], settings).observations[0].product.package
    assert package.model_dump() == {"quantity": quantity, "unit": unit, "count": 1}


def test_equivalent_sku_declarations_agree(packaged_products, settings):
    sku = packaged_products[0]["items"][0]
    sku.update(name="Café 0,5kg", nameComplete="Café 500g 0,5kg")
    page = parse(packaged_products[:1], settings)
    assert not page.issues
    assert page.observations[0].product.package.quantity == "0.5"


def test_conflict_retains_price_with_null_package_and_issue(packaged_products, settings):
    sku = packaged_products[0]["items"][0]
    sku.update(name="Café 400g", nameComplete="Café 500g")
    page = parse(packaged_products[:1], settings)
    assert page.observations[0].product.package is None
    assert page.observations[0].offers[0].basis == "package"
    assert "sku=215063: package-size:" in page.issues[0]
    assert "conflicting" in page.issues[0]


@pytest.mark.parametrize("quantity", ["0", "-1", "0,0000001", "1000000000"])
def test_invalid_contents_do_not_discard_valid_price(packaged_products, settings, quantity):
    rename(packaged_products[0], f"Café {quantity}g")
    page = parse(packaged_products[:1], settings)
    assert page.issues
    assert page.observations[0].product.package is None
    assert page.observations[0].offers


@pytest.mark.parametrize(
    "name",
    [
        "Café por 100 g",
        "Carne aprox. 500g",
        "Carne 400 a 600g",
        "Carne 400-600g",
        "Café 6x350ml",
        "Café 6 latas 350ml",
        "Peso bruto 500g",
        "Peso drenado 300g",
        "Porção 30g",
        "Café 500g + 50g",
        "Café sem tamanho",
        "Fralda Infantil G até 15kg",
        "Fralda Adulta para 70kg",
        "Saco para Lixo 50L",
        "Sacos de Lixo 100 L",
        "Saco Plástico para Lixo 30l",
    ],
)
def test_unknown_or_unverified_contents_stay_null(packaged_products, settings, name):
    rename(packaged_products[0], name)
    page = parse(packaged_products[:1], settings)
    assert not page.issues
    assert page.observations[0].product.package is None
    assert page.observations[0].offers


def test_parent_and_sibling_sizes_never_fill_missing_size(packaged_products, settings):
    product = packaged_products[0]
    sibling = copy.deepcopy(product["items"][0])
    sibling.update(itemId="sibling", name="Ovos 20un", nameComplete="Ovos 20un")
    product["items"].append(sibling)
    product.update(productName="Ovos 20un", description="Peso bruto 500g; porção 30g")
    rename(product, "Ovos sem tamanho")
    page = parse([product], settings, selected_sku="215063")
    assert not page.issues
    assert page.observations[0].product.package is None


def current_package(products, settings, name):
    rename(products[0], name)
    return parse(products[:1], settings).observations[0]


def test_current_size_changes_without_caching(packaged_products, settings):
    first = current_package(packaged_products, settings, "Café 500g")
    second = current_package(packaged_products, settings, "Café 400g")
    unknown = current_package(packaged_products, settings, "Café")
    assert first.product.package.quantity == "500"
    assert second.product.package.quantity == "400"
    assert unknown.product.package is None
    assert first.offers[0].amount == second.offers[0].amount == unknown.offers[0].amount
    assert first.product.source_product_id == second.product.source_product_id


def test_unit_spelling_is_equivalent_and_egg_count_can_change(packaged_products, settings):
    first = current_package(packaged_products, settings, "Café 0,5kg")
    second = current_package(packaged_products, settings, "Café 500g")
    assert equivalent_contents(first.product.package) == equivalent_contents(second.product.package)
    eggs = current_package(packaged_products, settings, "Ovos 12un")
    fewer_eggs = current_package(packaged_products, settings, "Ovos 10un")
    assert eggs.product.package.quantity == "12"
    assert fewer_eggs.product.package.quantity == "10"
