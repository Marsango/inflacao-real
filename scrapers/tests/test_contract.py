import json
from datetime import UTC, datetime
from uuid import uuid4

import pytest
from conftest import ROOT

from inflacao_scrapers.models import SCHEMA, Observation, Submission


@pytest.mark.parametrize("example", ["submission.json", "unavailable.json"])
def test_models_roundtrip_shared_contract_examples(example):
    path = ROOT / "docs/contracts/ingestion/v1/examples" / example
    document = json.loads(path.read_text())
    submission = Submission.model_validate_json(path.read_bytes())
    assert json.loads(submission.encode()) == document


def test_packaged_schema_matches_shared_source():
    assert SCHEMA == json.loads((ROOT / "docs/contracts/ingestion/v1/schema.json").read_text())


def test_naive_observation_time_rejected(observation):
    document = observation.model_dump()
    document["observed_at"] = datetime(2026, 10, 29)
    with pytest.raises(ValueError):
        Observation.model_validate(document)


@pytest.mark.parametrize(
    "change",
    [
        lambda d: d["product"].update(name=""),
        lambda d: d["product"].update(name="Product\x00name"),
        lambda d: d["product"].update(name="Bad\ud800name"),
        lambda d: d["context"].update(state="XX"),
        lambda d: d.update(product_url="https://user:secret@example.com/product"),
        lambda d: d["offers"][0].update(amount=16.66),
        lambda d: d["offers"][0].update(amount="NaN"),
        lambda d: d["offers"][0].update(reference_amount="0.01"),
        lambda d: d.update(extra="unknown"),
    ],
)
def test_bad_observations_do_not_become_plausible_data(observation, change):
    document = observation.model_dump(mode="json")
    change(document)
    with pytest.raises((ValueError, UnicodeError)):
        Observation.model_validate_json(json.dumps(document))


def test_duplicate_observation_ids_rejected(observation):
    with pytest.raises(ValueError, match="Duplicate"):
        Submission(
            schema_version="1.0",
            submission_id=uuid4(),
            scraper_id="scraper",
            shop_id="shop",
            run_id=uuid4(),
            created_at=datetime.now(UTC),
            collector={"name": "test", "version": "1"},
            observations=[observation, observation],
        )
