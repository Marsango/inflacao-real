"""Explicit adapter registration, independent of CLI and runner implementation."""

from ..angeloni.adapter import ADAPTER as ANGELONI_ADAPTER
from ..festval.adapter import ADAPTER as FESTVAL_ADAPTER
from ..muffato.adapter import ADAPTER as MUFFATO_ADAPTER
from . import Adapter

ADAPTERS: dict[str, Adapter] = {
    adapter.name: adapter for adapter in (MUFFATO_ADAPTER, ANGELONI_ADAPTER, FESTVAL_ADAPTER)
}


def registered_adapter(name: str) -> Adapter:
    if name not in ADAPTERS:
        available = ", ".join(sorted(ADAPTERS))
        raise ValueError(f"Unknown adapter {name!r}; registered adapters: {available}")
    return ADAPTERS[name]
