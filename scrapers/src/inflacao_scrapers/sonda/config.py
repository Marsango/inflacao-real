"""Public catalog settings do not assert a selected branch or pickup location."""

from typing import Literal

from pydantic import Field, field_validator

from ..config import Settings


class SondaSettings(Settings):
    adapter: Literal["sonda"] = "sonda"
    shop_id: Literal["sonda"] = "sonda"
    shop_url: Literal["https://www.sondadelivery.com.br"] = "https://www.sondadelivery.com.br"
    store_id: None = None
    city: None = None
    state: None = None
    max_catalog_pages: int = Field(default=600, ge=1, le=2000)
    category_paths: list[str] = Field(default_factory=list, max_length=100)

    @field_validator("category_paths")
    @classmethod
    def valid_paths(cls, paths):
        from .pages import category_path

        return list(dict.fromkeys(category_path(path) for path in paths))
