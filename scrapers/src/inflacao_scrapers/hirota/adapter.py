"""Hirota plugs into the shared monitoring, batching, delivery and spool runner."""

from ..adapters import Adapter, ParsedPage
from ..scope import MonitoringScope, MonitoringTarget
from .catalog import catalog_pages, listing_pages
from .config import CATALOG_PREFIX, HirotaSettings
from .session import prepare_context


def discover(requester, settings, query, limit):
    target = MonitoringTarget(
        id="discovery",
        name=query,
        search_queries=[query],
        required_terms=[[query]],
        excluded_terms=[],
    )
    scope = MonitoringScope(schema_version="1.0", targets=[target])
    result = ParsedPage()
    path = CATALOG_PREFIX + "produtos/mais_vendidos"
    pages = listing_pages(requester, settings, scope, path, [settings.max_catalog_pages])
    for page in pages:
        result.observations.extend(page.observations)
        result.issues.extend(page.issues)
        result.warnings.extend(page.warnings)
        if len(result.observations) >= limit:
            break
    result.observations = result.observations[:limit]
    return result


ADAPTER = Adapter(
    "hirota", HirotaSettings, None, discover, prepare=prepare_context, catalog_pages=catalog_pages
)
