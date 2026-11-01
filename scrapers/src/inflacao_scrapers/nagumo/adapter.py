"""Reuse the shared runner, monitoring scope, batching and durable delivery."""

from ..adapters import Adapter, ParsedPage
from ..scope import MonitoringTarget
from .catalog import search_pages
from .config import NagumoSettings
from .session import prepare_context


def discover(requester, settings, query, limit):
    target = MonitoringTarget(
        id="discovery",
        name=query,
        search_queries=[query],
        required_terms=[[query]],
        excluded_terms=[],
    )
    result = ParsedPage()
    pages = iter(search_pages(requester, settings, target, query))
    finished = False
    while not finished:
        finished = discover_page(pages, result, limit)
    result.observations = result.observations[:limit]
    return result


def discover_page(pages, result, limit):
    page = next(pages, None)
    if page is None:
        return True
    result.observations.extend(page.observations)
    result.issues.extend(page.issues)
    result.warnings.extend(page.warnings)
    return len(result.observations) >= limit


ADAPTER = Adapter("nagumo", NagumoSettings, search_pages, discover, prepare=prepare_context)
