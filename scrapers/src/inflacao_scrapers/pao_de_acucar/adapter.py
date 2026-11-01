"""Use the existing monitoring, batching, delivery and immutable spool pipeline."""

from ..adapters import Adapter, ParsedPage
from ..scope import MonitoringTarget
from .catalog import catalog_pages
from .config import PaoDeAcucarSettings
from .source import prepare_context


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
    "pao-de-acucar",
    PaoDeAcucarSettings,
    None,
    discover,
    prepare=prepare_context,
    catalog_pages=catalog_pages,
)
