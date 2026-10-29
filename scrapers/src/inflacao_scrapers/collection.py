"""Discover targets and deliver each page, preserving brands and failed batches."""

import sys
from dataclasses import dataclass, field
from uuid import UUID, uuid4

import httpx

from .delivery import batches
from .delivery.client import DeliveryTarget
from .delivery.immediate import DeliveryFailure, ImmediateDelivery
from .scope import fetch_scope
from .transport import RequestFailure


@dataclass
class CollectionRun:
    collector_name: str
    identity: UUID = field(default_factory=uuid4)
    observations: int = 0
    failures: int = 0
    seen: set[str] = field(default_factory=set)

    def issue(self, message: str) -> None:
        print(f"run_id={self.identity} {message}", file=sys.stderr)
        self.failures += 1

    def checkpoint(self, page, delivery, settings, target_id) -> set[str]:
        observations = [observation for observation in page.observations if self.keep(observation)]
        submissions = batches(
            observations,
            scraper_id=settings.scraper_id,
            shop_id=settings.shop_id,
            run_id=self.identity,
            collector_name=self.collector_name,
        )
        for submission in submissions:
            self.observations += len(submission.observations)
            delivery.submit(submission)
        for issue in page.issues:
            self.issue(f"target={target_id} {issue}")
        return {source_identity(observation) for observation in page.observations}

    def keep(self, observation) -> bool:
        identity = source_identity(observation)
        if identity in self.seen:
            return False
        self.seen.add(identity)
        return True

    def finish(self) -> int:
        print(
            f"run_id={self.identity} observations={self.observations} failures={self.failures} "
            f"complete={self.failures == 0}"
        )
        return int(self.failures != 0)


def collect_scope(requester, spool_factory, settings, token: str, adapter) -> int:
    scope = fetch_scope(requester, settings, token)
    run = CollectionRun(adapter.name)
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
