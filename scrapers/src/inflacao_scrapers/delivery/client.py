"""Authenticated delivery preserves bytes, IDs, and timestamps across retries."""

from dataclasses import dataclass

from ..models import Submission
from ..transport import Requester
from .spool import Spool


@dataclass
class DeliveryTarget:
    core_url: str
    token: str
    scraper_id: str
    shop_id: str

    def validate(self) -> None:
        if not self.token or any(character.isspace() for character in self.token):
            raise ValueError("INGESTION_TOKEN must be a nonempty token without whitespace")

    def authorize(self, payload: bytes) -> None:
        submission = Submission.model_validate_json(payload)
        if submission.scraper_id != self.scraper_id or submission.shop_id != self.shop_id:
            raise ValueError("Queued identity differs from configured scraper/shop; use its config")

    def send(self, requester: Requester, payload: bytes):
        return requester.request(
            "POST",
            self.core_url + "/api/v1/ingestion/submissions",
            content=payload,
            headers={"Authorization": f"Bearer {self.token}", "Content-Type": "application/json"},
        )


def require_acknowledgement(response, identity: str) -> None:
    if response.status_code not in (200, 202) or response.content:
        raise RuntimeError(
            f"Submission {identity}: unexpected HTTP {response.status_code}/body; "
            "batch retained for investigation"
        )


def deliver_batch(
    spool: Spool, requester: Requester, target: DeliveryTarget, queued, report
) -> None:
    identity, payload = queued
    target.authorize(payload)
    response = target.send(requester, payload)
    require_acknowledgement(response, identity)
    spool.acknowledge(identity)
    report(f"accepted submission_id={identity} status={response.status_code}")


def deliver(
    spool: Spool,
    requester: Requester,
    *,
    core_url: str,
    token: str,
    scraper_id: str,
    shop_id: str,
    report=print,
) -> int:
    target = DeliveryTarget(core_url, token, scraper_id, shop_id)
    target.validate()
    pending = list(spool.pending())
    for queued in pending:
        deliver_batch(spool, requester, target, queued, report)
    return len(pending)
