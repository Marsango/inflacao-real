"""Department recovery and overlap must preserve failed status and delivery safety."""

import json
import re

import httpx

from . import test_pipeline
from .test_pipeline import execute, shop_response, submission_requests
from .test_source import FIXTURES, source


def departments_response():
    return httpx.Response(200, json={"content": [
        {"uiLink": "/alimentos"}, {"uiLink": "/bebidas"},
    ]})


def recovery_response(request, failure):
    if request.url.path.endswith("categories"):
        return departments_response()
    if request.url.path.endswith("category-page"):
        return recovered_page(request)
    return shop_response(request, failure)


def recovered_page(request):
    body = json.loads(request.content)
    if body["multiCategory"] == "alimentos":
        return httpx.Response(404)
    return test_pipeline.single_page(request)


def test_failed_food_category_does_not_skip_later_departments(monkeypatch, tmp_path, capsys):
    monkeypatch.setattr(test_pipeline, "shop_response", recovery_response)
    status, requests = execute(tmp_path, monkeypatch)
    assert status == 1
    assert len(submission_requests(requests)) == 1
    output = capsys.readouterr()
    assert "category=alimentos: page=1:" in output.err
    assert "HTTP 404" in output.err
    assert "category=bebidas traversal_finished" in output.out
    assert "complete=False" in output.out


def test_delivery_failure_still_stops_before_next_department(monkeypatch, tmp_path):
    monkeypatch.setattr(test_pipeline, "shop_response", two_departments_response)
    status, requests = execute(tmp_path, monkeypatch, "delivery")
    categories = [json.loads(request.content)["multiCategory"] for request in requests
                  if request.url.path.endswith("category-page")]
    assert status == 1 and categories == ["alimentos"]


def two_departments_response(request, failure):
    if request.url.path.endswith("categories"):
        return departments_response()
    return shop_response(request, failure)


def overlap_response(request, failure):
    if request.url.path.endswith("category-page"):
        return overlap_page(json.loads(request.content)["page"])
    return shop_response(request, failure)


def overlap_page(number):
    document = catalog_document(number)
    if number == 2:
        document = replace_product_identities(document)
    return httpx.Response(200, content=document)


def replace_product_identities(document, skip_first=True):
    products = source("rice")["products"][int(skip_first):]
    for number, product in enumerate(products):
        pattern = r'("sku"\s*:\s*)"' + product["sku"] + '"'
        document = re.sub(pattern, rf'\g<1>"{9900000 + number}"', document)
    return document


def catalog_document(number, total=24):
    document = (FIXTURES / "pao-de-acucar-rice.json").read_text()
    document = re.sub(r'"page":\d+', f'"page":{number}', document)
    document = re.sub(r'"totalPages":\d+', f'"totalPages":{(total + 11) // 12}', document)
    return re.sub(r'"totalProducts":\d+', f'"totalProducts":{total}', document)


def test_overlap_delivers_only_one_measurement_per_listing(monkeypatch, tmp_path, capsys):
    monkeypatch.setattr(test_pipeline, "shop_response", overlap_response)
    status, requests = execute(tmp_path, monkeypatch)
    observations = submitted_observations(requests)
    assert status == 0 and len(observations) == 23
    assert len({item["product"]["source_product_id"] for item in observations}) == 23
    assert "overlapping_listings=1" in capsys.readouterr().err


def submitted_observations(requests):
    documents = [json.loads(request.content) for request in submission_requests(requests)]
    return [item for document in documents for item in document["observations"]]


def changed_total_response(request, failure):
    if request.url.path.endswith("categories"):
        return departments_response()
    if request.url.path.endswith("category-page"):
        return changed_total_page(request)
    return shop_response(request, failure)


def changed_total_page(request):
    body = json.loads(request.content)
    if body["multiCategory"] != "alimentos":
        document = test_pipeline.single_page(request).content.decode()
        return httpx.Response(200, content=replace_product_identities(document, skip_first=False))
    total = 24
    if body["page"] == 2:
        total = 25
    return httpx.Response(200, content=catalog_document(body["page"], total))


def test_changed_catalog_reports_page_and_collects_next_department(
    monkeypatch, tmp_path, capsys
):
    monkeypatch.setattr(test_pipeline, "shop_response", changed_total_response)
    status, requests = execute(tmp_path, monkeypatch)
    assert status == 1 and len(submission_requests(requests)) == 2
    assert "category=alimentos: page=2: GPA catalog changed" in capsys.readouterr().err
