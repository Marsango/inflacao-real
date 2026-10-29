import json
from pathlib import Path

import httpx
import pytest

from inflacao_scrapers.scope import MonitoringScope, MonitoringTarget, fetch_scope
from inflacao_scrapers.transport import Requester

EXAMPLES = json.loads(
    (Path(__file__).parents[2] / "docs/contracts/monitoring/v1/matching-examples.json").read_text()
)


@pytest.mark.parametrize("example", EXAMPLES, ids=lambda example: example["target"]["id"])
def test_shared_matching_examples(example):
    target = MonitoringTarget.model_validate(example["target"])
    for candidate in example["cases"]:
        assert target.matches(candidate["name"]) == candidate["matches"], candidate["name"]


@pytest.mark.parametrize("version", ["2.0", None, 1])
def test_unsupported_scope_versions_are_rejected(version):
    with pytest.raises(ValueError):
        MonitoringScope.model_validate({"schema_version": version, "targets": []})


def test_duplicate_target_identities_are_rejected():
    target = EXAMPLES[0]["target"]
    with pytest.raises(ValueError, match="Duplicate"):
        MonitoringScope.model_validate({"schema_version": "1.0", "targets": [target, target]})


@pytest.mark.parametrize("status", [301, 401, 403, 500])
def test_scope_failure_cannot_fall_back_to_local_product_list(settings, status):
    transport = httpx.MockTransport(lambda request: httpx.Response(status))
    with httpx.Client(transport=transport) as client, pytest.raises((ValueError, RuntimeError)):
        fetch_scope(Requester(client, attempts=1), settings, "secret")


def test_empty_enabled_scope_is_valid(settings):
    transport = httpx.MockTransport(
        lambda request: httpx.Response(200, json={"schema_version": "1.0", "targets": []})
    )
    with httpx.Client(transport=transport) as client:
        assert not fetch_scope(Requester(client), settings, "secret").targets
