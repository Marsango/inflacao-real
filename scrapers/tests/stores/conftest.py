import json
from dataclasses import dataclass
from pathlib import Path

import pytest

from inflacao_scrapers.adapters.registry import registered_adapter
from inflacao_scrapers.vtex.json import decode_response

FIXTURES = Path(__file__).parents[1] / "fixtures/stores"


@dataclass
class StoreSample:
    name: str
    adapter: object
    settings: object
    source: bytes
    regions: bytes
    categories: bytes = b"[]"

    def prepared_settings(self):
        region = decode_response(self.regions)[0]
        seller_id = self.settings.regional_seller_id
        return self.settings.model_copy(
            update={
                "region_id": region["id"],
                "store_id": f"region:{region['id']}/seller:{seller_id}",
            }
        )

    def products(self):
        data = decode_response(self.source)
        if isinstance(data, dict):
            return data["products"]
        return data


@pytest.fixture(params=["angeloni", "festval"])
def store_sample(request):
    return load_sample(request.param)


def load_sample(name):
    adapter = registered_adapter(name)
    model = adapter.settings_model
    sample = StoreSample(
        name,
        adapter,
        model(scraper_id="shared"),
        (FIXTURES / f"{name}-products.json").read_bytes(),
        (FIXTURES / f"{name}-region.json").read_bytes(),
    )
    if name == "angeloni":
        sample.categories = (FIXTURES / "angeloni-categories.json").read_bytes()
    return sample


def encode_source(document):
    return json.dumps(document, default=str, ensure_ascii=False).encode()


@pytest.fixture
def samples():
    return {name: load_sample(name) for name in ["angeloni", "festval"]}
