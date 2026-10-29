"""Festval Curitiba delivery context, resolved again at the start of each run."""

from pydantic import Field

from ..config import Identifier
from ..vtex.config import VtexSettings


class FestvalSettings(VtexSettings):
    adapter: str = "festval"
    shop_id: Identifier = "festval"
    shop_url: str = "https://www.festval.com"
    sales_channel: str = Field(default="2", pattern=r"^[0-9]+$")
    postal_code: str = Field(default="80730201", pattern=r"^[0-9]{8}$")
    regional_seller_id: str = Field(default="meufestvaldark", min_length=1)
    marketing_source: str = "curitiba"
    city: str = "Curitiba"
    state: str = "PR"
    store_id: str = "seller:meufestvaldark"
