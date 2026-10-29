import copy
import json
from decimal import Decimal

import httpx
import pytest

from inflacao_scrapers.adapters.registry import registered_adapter
from inflacao_scrapers.collection import collect_scope
from inflacao_scrapers.delivery import Spool
from inflacao_scrapers.scope import MonitoringTarget
from inflacao_scrapers.transport import Requester
from inflacao_scrapers.vtex.search import page_finished


@pytest.fixture
def target():
    return MonitoringTarget(
        id="coffee",
        name="Coffee",
        search_queries=["cafe", "cafe moido"],
        required_terms=[["cafe"], ["moido"]],
        excluded_terms=["soluvel"],
    )


def source_products(source_bytes, count, offset=0):
    template = json.loads(source_bytes, parse_float=Decimal)[0]
    return [source_product(template, number) for number in range(offset, offset + count)]


def source_product(template, number):
    product = copy.deepcopy(template)
    product["productId"] = str(number)
    product["brand"] = f"Brand {number}"
    sku = product["items"][0]
    sku["itemId"] = str(number)
    sku["nameComplete"] = f"Café Moído Brand {number} 500g"
    sku["name"] = sku["nameComplete"]
    return product


def collection_handler(target, pages, requests, status=200, delivery_status=202):
    def handler(request):
        requests.append(request)
        if request.url.path == "/api/v1/monitoring-targets":
            return httpx.Response(
                status, json={"schema_version": "1.0", "targets": [target.model_dump()]}
            )
        if request.method == "POST":
            assert request.headers["authorization"] == "Bearer secret"
            return httpx.Response(delivery_status)
        assert "authorization" not in request.headers
        offset = int(request.url.params["_from"])
        return source_response(pages[offset])

    return handler


def source_response(page):
    if isinstance(page, httpx.Response):
        return page
    if isinstance(page, int):
        return httpx.Response(page)
    return httpx.Response(206, content=json.dumps(page, default=str).encode())


def run_collection(tmp_path, settings, handler):
    submitted = []

    def recording_handler(request):
        if request.method == "POST":
            submitted.append(json.loads(request.content))
        return handler(request)

    with httpx.Client(transport=httpx.MockTransport(recording_handler)) as client:
        result = collect_scope(
            Requester(client, interval=0, attempts=1),
            lambda: Spool(tmp_path / "spool.sqlite3"),
            settings,
            "secret",
            registered_adapter(settings.adapter),
        )
    return result, submitted


def observations(pending):
    return [observation for submission in pending for observation in submission["observations"]]


def test_paginates_all_brands_and_deduplicates_overlapping_queries(
    tmp_path, settings, source_bytes, target
):
    requests = []
    pages = {0: source_products(source_bytes, 50), 50: source_products(source_bytes, 1, 50)}
    result, pending = run_collection(
        tmp_path, settings, collection_handler(target, pages, requests)
    )
    collected = observations(pending)
    assert result == 0
    assert len(collected) == 51
    assert len({observation["product"]["brand"] for observation in collected}) == 51
    source_requests = [request for request in requests if request.url.host != "127.0.0.1"]
    assert [request.url.params["_from"] for request in source_requests] == ["0", "50", "0", "50"]
    assert all(request.url.params["sc"] == "8" for request in source_requests)
    methods = [request.method for request in requests]
    assert methods == ["GET", "GET", "POST", "GET", "POST", "GET", "GET"]
    assert not (tmp_path / "spool.sqlite3").exists()


def test_nonmatching_names_are_filtered_before_offer_parsing(
    tmp_path, settings, source_bytes, target
):
    products = source_products(source_bytes, 2)
    products[1]["items"][0]["nameComplete"] = "Café Solúvel Moído 500g"
    products[1]["items"][0]["sellers"] = []
    handler = collection_handler(target, {0: products}, [])
    result, pending = run_collection(tmp_path, settings, handler)
    assert result == 0
    assert len(observations(pending)) == 1


def test_later_page_failure_keeps_prior_measurements(
    tmp_path, settings, source_bytes, target, capsys
):
    handler = collection_handler(target, {0: source_products(source_bytes, 50), 50: 403}, [])
    result, pending = run_collection(tmp_path, settings, handler)
    assert result == 1
    assert len(observations(pending)) == 50
    assert "collection incomplete" in capsys.readouterr().err


def test_missing_target_reports_incomplete_coverage(tmp_path, settings, target, capsys):
    result, pending = run_collection(tmp_path, settings, collection_handler(target, {0: []}, []))
    assert result == 1
    assert not pending
    assert "coverage missing" in capsys.readouterr().err


def test_pagination_limit_does_not_report_success(tmp_path, settings, source_bytes, target, capsys):
    settings = settings.model_copy(update={"max_search_pages": 1})
    handler = collection_handler(target, {0: source_products(source_bytes, 50)}, [])
    result, pending = run_collection(tmp_path, settings, handler)
    assert result == 1
    assert len(observations(pending)) == 50
    assert "page limit reached" in capsys.readouterr().err


def test_repeated_page_does_not_loop_or_report_success(
    tmp_path, settings, source_bytes, target, capsys
):
    products = source_products(source_bytes, 50)
    handler = collection_handler(target, {0: products, 50: products}, [])
    result, pending = run_collection(tmp_path, settings, handler)
    assert result == 1
    assert len(observations(pending)) == 50
    assert "repeated SKU page" in capsys.readouterr().err


def test_spool_exhaustion_stops_further_source_requests(
    tmp_path, settings, source_bytes, target, monkeypatch
):
    requests = []
    handler = collection_handler(
        target, {0: source_products(source_bytes, 50)}, requests, delivery_status=400
    )
    monkeypatch.setattr(Spool, "enqueue_payload", fail_enqueue)
    with pytest.raises(RuntimeError, match="full"):
        run_collection(tmp_path, settings, handler)
    assert len(requests) == 3


def fail_enqueue(self, identity, payload):
    raise RuntimeError("Spool full")


def test_failed_delivery_is_saved_before_stopping_collection(
    tmp_path, settings, source_bytes, target, capsys
):
    requests = []
    handler = collection_handler(
        target, {0: source_products(source_bytes, 50)}, requests, delivery_status=401
    )
    result, submitted = run_collection(tmp_path, settings, handler)
    assert result == 1
    assert len(requests) == 3
    assert len(submitted) == 1
    with Spool(tmp_path / "spool.sqlite3") as spool:
        identity, payload = next(spool.pending())
        assert payload == requests[-1].content
        assert identity == submitted[0]["submission_id"]
    output = capsys.readouterr()
    assert "saved locally" in output.err
    assert "complete=False" in output.out


def test_full_final_page_uses_source_total(tmp_path, settings, source_bytes, target):
    settings = settings.model_copy(update={"max_search_pages": 1})
    response = source_response(source_products(source_bytes, 50))
    response.headers["resources"] = "0-49/50"
    handler = collection_handler(target, {0: response}, [])
    result, pending = run_collection(tmp_path, settings, handler)
    assert result == 0
    assert len(observations(pending)) == 50


@pytest.mark.parametrize("header", ["broken", "50-99/100", "0-49/49"])
def test_invalid_source_ranges_cannot_report_complete(header):
    response = httpx.Response(206, headers={"resources": header})
    with pytest.raises(ValueError, match="incomplete"):
        page_finished(response, [{}] * 50, 0)


def test_short_page_with_more_source_results_is_not_complete():
    response = httpx.Response(206, headers={"resources": "0-9/20"})
    assert not page_finished(response, [{}] * 10, 0)


def test_final_page_header_can_describe_the_requested_range():
    response = httpx.Response(200, headers={"resources": "50-99/60"})
    assert page_finished(response, [{}] * 10, 50)
