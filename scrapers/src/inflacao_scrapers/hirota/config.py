"""Only the observed public Nazaré pickup context is supported."""

from typing import Literal

from pydantic import Field, PrivateAttr

from ..config import Settings

API_BASE = "https://services-beta.vipcommerce.com.br/api-admin/v1/"
CATALOG_PREFIX = "org/171/filial/1/centro_distribuicao/1/loja/"


class HirotaSettings(Settings):
    adapter: Literal["hirota"] = "hirota"
    shop_id: Literal["hirota"] = "hirota"
    shop_url: Literal["https://www.lojahirota.com.br"] = "https://www.lojahirota.com.br"
    store_id: Literal["org:171/filial:1/centro:1"] = "org:171/filial:1/centro:1"
    city: Literal["São Paulo"] = "São Paulo"
    state: Literal["SP"] = "SP"
    postal_code: Literal["04263100"] = "04263100"
    max_catalog_pages: int = Field(default=600, ge=1, le=1000)
    chrome_executable_path: str | None = None
    _session_headers: dict[str, str] = PrivateAttr(default_factory=dict)

    def bind_session(self, headers: dict[str, str]) -> None:
        self._session_headers = dict(headers)

    def source_headers(self) -> dict[str, str]:
        if not self._session_headers:
            raise ValueError("Hirota pickup context must be verified before collection")
        return {**self._session_headers, "Cache-Control": "no-cache", "Pragma": "no-cache"}
