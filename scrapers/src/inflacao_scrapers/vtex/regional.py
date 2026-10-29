"""Resolve the configured CEP and require its expected regional seller each run."""

from uuid import uuid4

import httpx
from pydantic import BaseModel, ConfigDict, Field, TypeAdapter

from ..transport import Requester
from .config import VtexSettings


class RegionalSeller(BaseModel):
    model_config = ConfigDict(strict=True)
    id: str = Field(min_length=1)
    name: str


class Region(BaseModel):
    model_config = ConfigDict(strict=True)
    id: str = Field(min_length=1, max_length=256)
    sellers: list[RegionalSeller]


def prepare_region(requester: Requester, settings: VtexSettings) -> VtexSettings:
    response = request_region(requester, settings)
    if response.status_code != 200:
        raise ValueError(f"Regional context HTTP {response.status_code}; collection not started")
    regions = TypeAdapter(list[Region]).validate_json(response.content)
    region = select_region(regions, settings.regional_seller_id)
    return settings.model_copy(
        update={
            "region_id": region.id,
            "store_id": f"region:{region.id}/seller:{settings.regional_seller_id}",
        }
    )


def request_region(requester: Requester, settings: VtexSettings) -> httpx.Response:
    response = requester.request(
        "GET",
        settings.shop_url + "/api/checkout/pub/regions",
        params={
            "country": "BRA",
            "postalCode": settings.postal_code,
            "sc": settings.sales_channel,
            "_ir": str(uuid4()),
        },
        headers={"Cache-Control": "no-cache, max-age=0"},
    )
    return response


def select_region(regions: list[Region], seller_id: str | None) -> Region:
    matching = [region for region in regions if contains_seller(region, seller_id)]
    if len(matching) != 1:
        raise ValueError("Configured regional seller absent or ambiguous; collection not started")
    return matching[0]


def contains_seller(region: Region, seller_id: str | None) -> bool:
    return any(seller.id == seller_id for seller in region.sellers)
