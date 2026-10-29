"""Discover every target on each run, preserving brands and durable page checkpoints."""

import sys
from dataclasses import dataclass, field
from uuid import UUID, uuid4

import httpx

from .delivery import batches
from .muffato.search import search_pages
from .scope import fetch_scope
from .transport import RequestFailure


@dataclass
class CollectionRun:
    identity: UUID = field(default_factory=uuid4)
    observations: int = 0
    failures: int = 0
    seen: set[str] = field(default_factory=set)

    def issue(self, message: str) -> None:
        print(f"run_id={self.identity} {message}", file=sys.stderr)
        self.failures += 1

    def checkpoint(self, page, spool, settings, target_id) -> set[str]:
        observations = [observation for observation in page.observations if self.keep(observation)]
        submissions = batches(
            observations,
            scraper_id=settings.scraper_id,
            shop_id=settings.shop_id,
            run_id=self.identity,
        )
        for submission in submissions:
            spool.enqueue(submission)
            self.observations += len(submission.observations)
            print(f"queued submission_id={submission.submission_id} target={target_id}", flush=True)
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


def collect_scope(requester, spool, settings, token: str) -> int:
    scope = fetch_scope(requester, settings, token)
    run = CollectionRun()
    print(f"run_id={run.identity} expected_targets={len(scope.targets)}", flush=True)
    for target in scope.targets:
        collect_target(run, requester, spool, settings, target)
    return run.finish()


def collect_target(run, requester, spool, settings, target) -> None:
    try:
        count = collect_queries(run, requester, spool, settings, target)
        report_target(run, target.id, count)
    except (httpx.HTTPError, ValueError, RequestFailure) as error:
        run.issue(f"target={target.id}: {error}")


def collect_queries(run, requester, spool, settings, target) -> int:
    identities = set()
    for query in target.search_queries:
        identities.update(collect_query(run, requester, spool, settings, target, query))
    return len(identities)


def collect_query(run, requester, spool, settings, target, query) -> set[str]:
    identities = set()
    for page in search_pages(requester, settings, target, query):
        identities.update(run.checkpoint(page, spool, settings, target.id))
    return identities


def source_identity(observation) -> str:
    product = observation.product
    return product.source_product_id


def report_target(run, identity: str, count: int) -> None:
    print(f"run_id={run.identity} target={identity} matched_listings={count}", flush=True)
    if count == 0:
        run.issue(f"target={identity}: no matching listings; coverage missing")
