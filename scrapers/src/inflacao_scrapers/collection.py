"""Discover targets and deliver each page, preserving brands and failed batches."""

import sys
from dataclasses import dataclass, field
from datetime import UTC, datetime
from uuid import UUID, uuid4

import httpx

from .delivery import batches
from .delivery.client import DeliveryTarget
from .delivery.immediate import DeliveryFailure, ImmediateDelivery
from .diagnostics.report import record
from .history_refresh import request_history_refresh
from .scope import fetch_scope
from .transport import RequestFailure


@dataclass
class CollectionRun:
    collector_name: str
    identity: UUID = field(default_factory=uuid4)
    observations: int = 0
    failures: int = 0
    warnings: int = 0
    unknown_packages: int = 0
    started_at: datetime = field(default_factory=lambda: datetime.now(UTC))
    diagnostics: list[dict] = field(default_factory=list)
    available_prices: int = 0
    unavailable: int = 0
    seen: set[str] = field(default_factory=set)

    def issue(self, message: str) -> None:
        print(f"run_id={self.identity} {message}", file=sys.stderr)
        self.failures += 1
        record(self, "error", message)

    def warning(self, message: str) -> None:
        print(f"run_id={self.identity} warning: {message}", file=sys.stderr)
        self.warnings += 1
        record(self, "warning", message)

    def checkpoint(self, page, delivery, settings, target_id) -> set[str]:
        return checkpoint(self, page, delivery, settings, target_id)

    def keep(self, observation) -> bool:
        identity = source_identity(observation)
        if identity in self.seen:
            return False
        self.seen.add(identity)
        return True

    def finish(self) -> int:
        print(
            f"run_id={self.identity} observations={self.observations} failures={self.failures} "
            f"warnings={self.warnings} unknown_packages={self.unknown_packages} "
            f"available_price_observations={self.available_prices} "
            f"unavailable_observations={self.unavailable} "
            f"complete={self.failures == 0}"
        )
        return int(self.failures != 0)


def checkpoint(run, page, delivery, settings, target_id):
    observations = [observation for observation in page.observations if run.keep(observation)]
    run.unknown_packages += sum(unknown_package(observation) for observation in observations)
    run.available_prices += sum(available_price(item) for item in observations)
    run.unavailable += sum(item.availability == "out_of_stock" for item in observations)
    submissions = collection_batches(observations, settings, run)
    for submission in submissions:
        run.observations += len(submission.observations)
        delivery.submit(submission)
    report_page(run, page, target_id)
    return {source_identity(observation) for observation in page.observations}


def collection_batches(observations, settings, run):
    return batches(
        observations,
        scraper_id=settings.scraper_id,
        shop_id=settings.shop_id,
        run_id=run.identity,
        collector_name=run.collector_name,
    )


def available_price(observation) -> bool:
    return observation.availability == "in_stock" and bool(observation.offers)


def unknown_package(observation) -> bool:
    if observation.product.package is not None:
        return False
    return any(offer.basis == "package" for offer in observation.offers)


def collect_scope(requester, spool_factory, settings, token: str, adapter, run=None) -> int:
    run = run or CollectionRun(adapter.name)
    scope = fetch_scope(requester, settings, token)
    print(f"run_id={run.identity} expected_targets={len(scope.targets)}", flush=True)
    if not scope.targets:
        return run.finish()
    settings = adapter.prepare(requester, settings)
    destination = DeliveryTarget(settings.core_url, token, settings.scraper_id, settings.shop_id)
    delivery = ImmediateDelivery(requester, destination, spool_factory)
    try:
        collect_adapter(run, requester, delivery, settings, scope, adapter)
    except (DeliveryFailure, ValueError, RequestFailure, httpx.HTTPError) as error:
        run.issue(str(error))
    request_history_refresh(requester, settings, token)
    return run.finish()


def collect_adapter(run, requester, delivery, settings, scope, adapter) -> None:
    if adapter.catalog_pages is not None:
        collect_catalog(run, requester, delivery, settings, scope, adapter)
        return
    collect_targets(run, requester, delivery, settings, scope.targets, adapter)


def collect_catalog(run, requester, delivery, settings, scope, adapter) -> None:
    coverage = {target.id: set() for target in scope.targets}
    for page in adapter.catalog_pages(requester, settings, scope):
        run.checkpoint(page, delivery, settings, "catalog")
        update_coverage(coverage, page.observations, scope.targets)
    for identity, listings in coverage.items():
        report_target(run, identity, len(listings))


def update_coverage(coverage, observations, targets) -> None:
    for target in targets:
        coverage[target.id].update(matching_identities(observations, target))


def matching_identities(observations, target) -> set[str]:
    return {
        source_identity(observation)
        for observation in observations
        if target.matches(observation.product.name)
    }


def collect_targets(run, requester, delivery, settings, targets, adapter) -> None:
    for target in targets:
        collect_target(run, requester, delivery, settings, target, adapter)


def collect_target(run, requester, delivery, settings, target, adapter) -> None:
    try:
        count = collect_queries(run, requester, delivery, settings, target, adapter)
        report_target(run, target.id, count)
    except (httpx.HTTPError, ValueError, RequestFailure) as error:
        run.issue(f"target={target.id}: {error}")


def collect_queries(run, requester, delivery, settings, target, adapter) -> int:
    identities = set()
    for query in target.search_queries:
        identities.update(collect_query(run, requester, delivery, settings, target, query, adapter))
    return len(identities)


def collect_query(run, requester, delivery, settings, target, query, adapter) -> set[str]:
    identities = set()
    for page in adapter.search_pages(requester, settings, target, query):
        identities.update(run.checkpoint(page, delivery, settings, target.id))
    return identities


def source_identity(observation) -> str:
    product = observation.product
    return product.source_product_id


def report_target(run, identity: str, count: int) -> None:
    print(f"run_id={run.identity} target={identity} matched_listings={count}", flush=True)
    if count == 0:
        run.issue(f"target={identity}: no matching listings; coverage missing")


def report_page(run: CollectionRun, page, target_id: str) -> None:
    for issue in page.issues:
        run.issue(f"target={target_id} {issue}")
    for warning in page.warnings:
        run.warning(f"target={target_id} {warning}")
