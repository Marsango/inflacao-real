"""Sonda uses shared monitoring coverage, deduplication, batching and durable delivery."""

from ..adapters import Adapter, ParsedPage
from ..scope import MonitoringTarget
from .catalog import catalog_pages, prepare
from .config import SondaSettings


def discover(requester, settings, query, limit):
    target = MonitoringTarget(
        id="discovery",
        name=query,
        search_queries=[query],
        required_terms=[[query]],
        excluded_terms=[],
    )
    result = ParsedPage()
    for page in catalog_pages(requester, settings, target):
        result.observations.extend(page.observations)
        result.issues.extend(page.issues)
        result.warnings.extend(page.warnings)
        if len(result.observations) >= limit:
            break
    result.observations = result.observations[:limit]
    return result


ADAPTER = Adapter(
    "sonda", SondaSettings, None, discover, prepare=prepare, catalog_pages=catalog_pages
)
