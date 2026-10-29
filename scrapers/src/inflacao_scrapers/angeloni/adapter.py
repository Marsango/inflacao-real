"""Discover catalog categories, filter core targets, and use shared delivery."""

from ..adapters import Adapter, ParsedPage
from ..scope import MonitoringScope, MonitoringTarget
from ..transport import Requester
from ..vtex.catalog import catalog_pages as browse_catalog
from ..vtex.regional import prepare_region
from .categories import catalog_partitions
from .config import AngeloniSettings


def catalog_pages(requester, settings, scope):
    partitions = catalog_partitions(requester, settings)
    yield from browse_catalog(requester, settings, scope, partitions)


def discover(
    requester: Requester, settings: AngeloniSettings, query: str, limit: int
) -> ParsedPage:
    target = MonitoringTarget(
        id="discovery",
        name=query,
        search_queries=[query],
        required_terms=[[query]],
        excluded_terms=[],
    )
    scope = MonitoringScope(schema_version="1.0", targets=[target])
    result = ParsedPage()
    pages = catalog_pages(requester, settings, scope)
    page = next(pages, None)
    while page is not None:
        result.observations.extend(page.observations)
        result.issues.extend(page.issues)
        page = next_discovery_page(pages, result, limit)
    result.observations = result.observations[:limit]
    return result


def next_discovery_page(pages, result, limit):
    if len(result.observations) >= limit:
        return None
    return next(pages, None)


ADAPTER = Adapter(
    "angeloni",
    AngeloniSettings,
    None,
    discover,
    prepare=prepare_region,
    catalog_pages=catalog_pages,
)
