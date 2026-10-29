"""Shop adapters supply parsed pages; shared orchestration owns delivery."""

from collections.abc import Callable, Iterator
from dataclasses import dataclass, field

from ..config import Settings
from ..models import Observation
from ..scope import MonitoringScope, MonitoringTarget
from ..transport import Requester


@dataclass
class ParsedPage:
    observations: list[Observation] = field(default_factory=list)
    issues: list[str] = field(default_factory=list)


def unchanged_settings(requester, settings):
    return settings


@dataclass(frozen=True)
class Adapter:
    name: str
    settings_model: type[Settings]
    search_pages: (
        Callable[[Requester, Settings, MonitoringTarget, str], Iterator[ParsedPage]] | None
    )
    discover: Callable[[Requester, Settings, str, int], ParsedPage]
    prepare: Callable[[Requester, Settings], Settings] = unchanged_settings
    catalog_pages: Callable[[Requester, Settings, MonitoringScope], Iterator[ParsedPage]] | None = (
        None
    )
