"""Limit collection to the branch verified against the public storefront."""

from typing import Literal

from pydantic import Field, PrivateAttr

from ..config import Settings

ROUTE_PREFIX = "/on/demandware.store/Sites-Nagumo-Site/pt_BR/"
PAGE_SIZE = 24


class NagumoSettings(Settings):
    adapter: Literal["nagumo"] = "nagumo"
    shop_id: Literal["nagumo"] = "nagumo"
    shop_url: Literal["https://www.nagumo.com.br"] = "https://www.nagumo.com.br"
    store_id: Literal["13"] = "13"
    postal_code: Literal["08030040"] = "08030040"
    city: Literal["São Paulo"] = "São Paulo"
    state: Literal["SP"] = "SP"
    timeout_seconds: float = Field(default=30.0, gt=0, le=120)
    max_search_pages: int = Field(default=100, ge=1, le=1000)
    _context_verified: bool = PrivateAttr(default=False)

    def confirm_context(self) -> None:
        self._context_verified = True

    def require_context(self) -> None:
        if not self._context_verified:
            raise ValueError("Nagumo pickup context must be verified before collection")
