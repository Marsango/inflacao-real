"""One independently resolved São Paulo delivery catalog."""

from typing import Literal

from pydantic import Field, PrivateAttr

from ..config import Settings

API_BASE = "https://api.vendas.gpa.digital/pa/"


class PaoDeAcucarSettings(Settings):
    adapter: Literal["pao-de-acucar"] = "pao-de-acucar"
    shop_id: Literal["pao-de-acucar"] = "pao-de-acucar"
    shop_url: Literal["https://www.paodeacucar.com"] = "https://www.paodeacucar.com"
    store_id: Literal["461"] = "461"
    postal_code: Literal["04115090"] = "04115090"
    city: Literal["São Paulo"] = "São Paulo"
    state: Literal["SP"] = "SP"
    max_catalog_pages: int = Field(default=2000, ge=1, le=3000)
    _context_verified: bool = PrivateAttr(default=False)

    def verify_context(self) -> None:
        self._context_verified = True

    def require_context(self) -> None:
        if not self._context_verified:
            raise ValueError("Pão de Açúcar delivery context must be resolved before collection")
