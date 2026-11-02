def test_each_scraper_requests_refresh_after_collection(
    run_scrapers, history_refresh_notifications
):
    execution = run_scrapers()
    assert execution.status == 0
    assert [shop for shop, _ in history_refresh_notifications] == ["super-muffato", "fixture-shop"]


def test_partial_collection_still_refreshes_committed_data(
    run_scrapers, history_refresh_notifications
):
    execution = run_scrapers(failures={"www.supermuffato.com.br": 403})
    assert execution.status == 1
    assert [shop for shop, _ in history_refresh_notifications] == ["super-muffato", "fixture-shop"]
