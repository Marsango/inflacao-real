"""Strict model defaults and exact wire amounts."""

from typing import Annotated

from pydantic import BaseModel, ConfigDict, Field

Money = Annotated[str, Field(pattern=r"^(?:0|[1-9][0-9]{0,8})\.[0-9]{2}$")]


class WireModel(BaseModel):
    model_config = ConfigDict(extra="forbid", strict=True, frozen=True)
