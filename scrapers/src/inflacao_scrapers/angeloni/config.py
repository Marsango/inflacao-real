"""Angeloni Bigorrilho delivery context and full-catalog browsing limits."""

from pydantic import Field

from ..config import Identifier
from ..vtex.config import VtexSettings


class AngeloniSettings(VtexSettings):
    adapter: str = "angeloni"
    shop_id: Identifier = "angeloni"
    shop_url: str = "https://super.angeloni.com.br"
    sales_channel: str = Field(default="1", pattern=r"^[0-9]+$")
    postal_code: str = Field(default="80730201", pattern=r"^[0-9]{8}$")
    regional_seller_id: str = Field(default="superangeloni24", min_length=1)
    city: str = "Curitiba"
    state: str = "PR"
    store_id: str = "seller:superangeloni24"
    catalog_category_ids: list[int] = Field(
        default_factory=lambda: [
            43,
            240,
            274,
            281,
            283,
            255,
            390,
            146,
            244,
            260,
            183,
            185,
            84,
            226,
            263,
        ],
        min_length=1,
        max_length=200,
    )

    def supports_kilogram_prices(self) -> bool:
        return (
            self.sales_channel == "1"
            and self.seller_id == "1"
            and self.regional_seller_id == "superangeloni24"
            and self.region_id is not None
            and self.city == "Curitiba"
            and self.state == "PR"
        )
