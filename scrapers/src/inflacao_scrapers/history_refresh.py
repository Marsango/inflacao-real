"""Request asynchronous recalculation after collection or spool delivery ends."""

import sys

import httpx

from .transport import RequestFailure


def request_history_refresh(requester, settings, token: str) -> None:
    try:
        response = requester.request(
            "POST",
            settings.core_url + "/api/v1/ingestion/history-refresh",
            headers={"Authorization": f"Bearer {token}"},
            json={"scraper_id": settings.scraper_id, "shop_id": settings.shop_id},
        )
        if response.status_code != 202:
            raise ValueError(f"History refresh HTTP {response.status_code}")
    except (httpx.HTTPError, RequestFailure, ValueError):
        print(
            f"shop_id={settings.shop_id}: history refresh notification failed; "
            "the core will retry precomputation after ingestion drains",
            file=sys.stderr,
        )
