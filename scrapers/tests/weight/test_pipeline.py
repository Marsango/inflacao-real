from test_collection import collection_handler, observations, run_collection

from inflacao_scrapers.scope import MonitoringTarget

from .conftest import rename


def test_weighted_and_unsupported_results_deliver_valid_prices_and_fail_completeness(
    tmp_path, settings, weighted_products, capsys
):
    weighted_products[1]["items"][0]["measurementUnit"] = "g"
    target = MonitoringTarget(
        id="food",
        name="Food",
        search_queries=["food"],
        required_terms=[["banana", "patinho", "pao"]],
        excluded_terms=[],
    )
    handler = collection_handler(target, {0: weighted_products}, [])
    status, submissions = run_collection(tmp_path, settings, handler)
    assert status == 1
    collected = observations(submissions)
    assert len(collected) == 3
    assert collected[0]["product"]["package"] is None
    assert collected[0]["offers"][0]["basis"] == "kg"
    assert collected[0]["offers"][0]["amount"] == "13.40"
    assert "sale-format" in capsys.readouterr().err


def test_missing_contents_are_visible_without_invalidating_package_price(
    tmp_path, settings, weighted_products, capsys
):
    product = weighted_products[3]
    rename(product, "Carne Patinho")
    target = MonitoringTarget(
        id="beef",
        name="Beef",
        search_queries=["patinho"],
        required_terms=[["patinho"]],
        excluded_terms=[],
    )
    status, submissions = run_collection(
        tmp_path, settings, collection_handler(target, {0: [product]}, [])
    )
    assert status == 0
    assert observations(submissions)[0]["product"]["package"] is None
    assert "unknown_packages=1" in capsys.readouterr().out
