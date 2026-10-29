"""Non-secret TOML configuration; ingestion tokens come only from the environment."""

import tomllib
from pathlib import Path
from typing import Self

from pydantic import BaseModel, ConfigDict, Field, field_validator, model_validator

from .models import validate_url


class Settings(BaseModel):
    model_config = ConfigDict(extra="forbid", strict=True)

    scraper_id: str = Field(pattern=r"^[a-z0-9]+(?:[-_][a-z0-9]+)*$", max_length=64)
    shop_id: str = "super-muffato"
    shop_url: str = "https://www.supermuffato.com.br"
    core_url: str = "http://127.0.0.1:8080"
    max_search_pages: int = Field(default=50, ge=1, le=50)
    seller_id: str = "1"
    sales_channel: str = "1"
    store_id: str | None = None
    city: str | None = None
    state: str | None = None
    spool_path: str = "data/spool.sqlite3"
    spool_capacity_bytes: int = Field(default=64 * 1024 * 1024, ge=1048576)
    timeout_seconds: float = Field(default=20.0, gt=0, le=120)
    request_interval_seconds: float = Field(default=1.0, ge=0.5, le=60)
    max_attempts: int = Field(default=3, ge=1, le=5)
    retry_base_seconds: float = Field(default=1.0, gt=0, le=60)
    retry_max_seconds: float = Field(default=60.0, gt=0, le=300)

    @field_validator("shop_url", "core_url")
    @classmethod
    def valid_url(cls, value: str) -> str:
        from urllib.parse import urlsplit

        validate_url(value)
        parts = urlsplit(value)
        if parts.query or parts.fragment or parts.path not in ("", "/"):
            raise ValueError("Base URLs must contain only a scheme and host/port")
        if parts.scheme != "https" and parts.hostname not in ("localhost", "127.0.0.1", "::1"):
            raise ValueError("Use HTTPS outside local development")
        return value.rstrip("/")

    @model_validator(mode="after")
    def scope(self) -> Self:
        if not self.seller_id or not self.sales_channel.isdigit():
            raise ValueError("Seller and sales channel must be explicit")
        if (self.city is not None or self.state is not None) and self.store_id is None:
            raise ValueError("City/state require an explicitly selected store")
        return self


def load_settings(path: Path) -> Settings:
    with path.open("rb") as file:
        return Settings.model_validate(tomllib.load(file))
