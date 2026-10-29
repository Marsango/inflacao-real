import json
from pathlib import Path

import httpx
import pytest

from inflacao_scrapers.scope import MonitoringTarget
from inflacao_scrapers.transport import Requester
from inflacao_scrapers.vtex.search import search_pages

EXAMPLES = json.loads(
    (Path(__file__).parents[2] / "docs/contracts/monitoring/v1/matching-examples.json").read_text()
)
EXAMPLES = [
    example
    for example in EXAMPLES
    if example["target"]["id"] in {"arroz-branco-tipo-1", "cafe-moido"}
]


@pytest.mark.live
@pytest.mark.parametrize("example", EXAMPLES, ids=lambda example: example["target"]["id"])
def test_live_target_search_and_pagination(settings, example):
    target = MonitoringTarget.model_validate(example["target"])
    with httpx.Client(timeout=20, follow_redirects=False) as client:
        pages = list(search_pages(Requester(client), settings, target, target.search_queries[0]))
    assert any(page.observations for page in pages), "No matching listings; coverage missing"
    assert not [issue for page in pages for issue in page.issues]
