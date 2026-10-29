"""Non-secret TOML configuration; ingestion tokens come only from the environment."""

import tomllib
from pathlib import Path
from typing import Annotated, Self

from pydantic import BaseModel, ConfigDict, Field, field_validator, model_validator

from .models import validate_url

Identifier = Annotated[str, Field(pattern=r"^[a-z0-9]+(?:[-_][a-z0-9]+)*$", max_length=64)]


class Settings(BaseModel):
    model_config = ConfigDict(extra="forbid", strict=True)

    adapter: str = "super-muffato"
    enabled: bool = True
    scraper_id: Identifier
    shop_id: Identifier
    shop_url: str
    core_url: str = "http://127.0.0.1:8080"
    ingestion_token_env: str = Field(default="INGESTION_TOKEN", pattern=r"^[A-Za-z_][A-Za-z0-9_]*$")
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
        if (self.city is not None or self.state is not None) and self.store_id is None:
            raise ValueError("City/state require an explicitly selected store")
        return self


class RunnerConfiguration(BaseModel):
    model_config = ConfigDict(extra="forbid", strict=True)
    scrapers: list[dict] = Field(min_length=1)


class ScraperSelection(BaseModel):
    model_config = ConfigDict(extra="ignore", strict=True)
    adapter: str = "super-muffato"
    enabled: bool = True


def configuration_values(path: Path) -> tuple[list[dict], bool]:
    with path.open("rb") as file:
        document = tomllib.load(file)
    if "scrapers" in document:
        return RunnerConfiguration.model_validate(document).scrapers, True
    return [document], False


def resolve_settings(values: dict, multiple: bool) -> Settings | None:
    from .adapters.registry import registered_adapter

    selection = ScraperSelection.model_validate(values)
    if not selection.enabled:
        return None
    adapter = registered_adapter(selection.adapter)
    settings_model = adapter.settings_model
    settings = settings_model.model_validate(values)
    if multiple and "spool_path" not in values:
        settings.spool_path = f"data/{settings.shop_id}/{settings.scraper_id}/spool.sqlite3"
    return settings


def load_configurations(path: Path) -> list[Settings]:
    values, multiple = configuration_values(path)
    resolved = [resolve_settings(value, multiple) for value in values]
    settings = [setting for setting in resolved if setting is not None]
    paths = [(path.parent / setting.spool_path).resolve() for setting in settings]
    if len(paths) != len(set(paths)):
        raise ValueError("Enabled scrapers must use distinct spool_path values")
    return settings


def load_settings(path: Path) -> Settings:
    settings = load_configurations(path)
    if len(settings) != 1:
        raise ValueError("Expected exactly one enabled scraper configuration")
    return settings[0]
