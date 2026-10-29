"""Common VTEX request limits and explicit regional context."""

import base64
import json
from typing import Self

from pydantic import Field, model_validator

from ..config import Settings


class VtexSettings(Settings):
    max_search_pages: int = Field(default=50, ge=1, le=50)
    max_catalog_pages: int = Field(default=1000, ge=1, le=2000)
    seller_id: str = Field(default="1", min_length=1)
    sales_channel: str = Field(default="1", pattern=r"^[0-9]+$")
    postal_code: str | None = Field(default=None, pattern=r"^[0-9]{8}$")
    regional_seller_id: str | None = None
    region_id: str | None = None
    marketing_source: str | None = None

    def supports_kilogram_prices(self) -> bool:
        return False

    def supports_egg_counts(self) -> bool:
        return False

    @model_validator(mode="after")
    def delivery_context(self) -> Self:
        if self.postal_code and not self.regional_seller_id:
            raise ValueError("A delivery postal_code requires an explicit regional_seller_id")
        return self

    def observation_channel(self) -> str:
        if self.postal_code is not None:
            return "delivery"
        return "online"

    def region_parameters(self) -> dict:
        self.require_region()
        if self.region_id is not None:
            return {"regionId": self.region_id}
        return {}

    def source_headers(self) -> dict:
        self.require_region()
        headers = {"Cache-Control": "no-cache, max-age=0", "Pragma": "no-cache"}
        if self.region_id is not None:
            headers["Cookie"] = "vtex_segment=" + segment_cookie(self)
        return headers

    def require_region(self) -> None:
        if self.postal_code is not None and self.region_id is None:
            raise ValueError("Regional context must be verified before collecting prices")


def segment_cookie(settings: VtexSettings) -> str:
    segment = {
        "channel": settings.sales_channel,
        "regionId": settings.region_id,
        "countryCode": "BRA",
        "cultureInfo": "pt-BR",
        "currencyCode": "BRL",
        "utm_source": settings.marketing_source,
    }
    document = json.dumps(segment, separators=(",", ":")).encode()
    return base64.b64encode(document).decode()
