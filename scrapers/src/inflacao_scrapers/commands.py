"""Command workflows: discovery, automatic delivery, and failed-batch recovery."""

import json
import os
import sys
from functools import partial

import httpx

from .collection import collect_scope
from .config import Settings
from .delivery import Spool, deliver
from .history_refresh import request_history_refresh
from .transport import Requester


def create_client(settings: Settings) -> httpx.Client:
    return httpx.Client(
        timeout=settings.timeout_seconds,
        follow_redirects=False,
        limits=httpx.Limits(max_connections=1, max_keepalive_connections=1),
        headers={"User-Agent": "NossaInflacao/0.1 price-observation collector"},
    )


def create_requester(client: httpx.Client, settings: Settings) -> Requester:
    return Requester(
        client,
        attempts=settings.max_attempts,
        interval=settings.request_interval_seconds,
        retry_base=settings.retry_base_seconds,
        retry_max=settings.retry_max_seconds,
    )


def create_spool(settings: Settings, arguments) -> Spool:
    return Spool(arguments.config.parent / settings.spool_path, settings.spool_capacity_bytes)


def status_command(settings: Settings, arguments, adapter) -> int:
    with create_spool(settings, arguments) as spool:
        count, size = spool.status()
        print(f"pending_batches={count} payload_bytes={size}")
    return 0


def discover_command(settings: Settings, arguments, adapter) -> int:
    with create_client(settings) as client:
        requester = create_requester(client, settings)
        settings = adapter.prepare(requester, settings)
        page = adapter.discover(requester, settings, arguments.query, arguments.limit)
    return report_discovery(page)


def report_discovery(page) -> int:
    for observation in page.observations:
        print(json.dumps(observation.summary(), ensure_ascii=False))
    for issue in page.issues:
        print(issue, file=sys.stderr)
    for warning in page.warnings:
        print(f"warning: {warning}", file=sys.stderr)
    return int(bool(page.issues))


def deliver_command(settings: Settings, arguments, adapter) -> int:
    with create_client(settings) as client, create_spool(settings, arguments) as spool:
        requester = create_requester(client, settings)
        count = deliver(
            spool,
            requester,
            core_url=settings.core_url,
            token=os.environ.get(settings.ingestion_token_env, ""),
            scraper_id=settings.scraper_id,
            shop_id=settings.shop_id,
        )
        token = os.environ.get(settings.ingestion_token_env, "")
        request_history_refresh(requester, settings, token)
    print(f"accepted_batches={count}; background processing must be checked in core")
    return 0


def collect_command(settings: Settings, arguments, adapter) -> int:
    with create_client(settings) as client:
        requester = create_requester(client, settings)
        spool_factory = partial(create_spool, settings, arguments)
        return collect_scope(
            requester,
            spool_factory,
            settings,
            os.environ.get(settings.ingestion_token_env, ""),
            adapter,
        )


COMMANDS = {
    "status": status_command,
    "discover": discover_command,
    "deliver": deliver_command,
    "collect": collect_command,
}
