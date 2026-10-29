"""Send new batches immediately; open the local spool only after delivery fails."""

from collections.abc import Callable
from contextlib import AbstractContextManager
from dataclasses import dataclass
from typing import NoReturn

import httpx

from ..models import Submission
from ..transport import Requester, RequestFailure
from .client import DeliveryRejected, DeliveryTarget, send_payload
from .spool import Spool


class DeliveryFailure(RuntimeError):
    pass


@dataclass
class ImmediateDelivery:
    requester: Requester
    target: DeliveryTarget
    spool_factory: Callable[[], AbstractContextManager[Spool]]

    def submit(self, submission: Submission) -> None:
        identity = str(submission.submission_id)
        payload = submission.encode()
        try:
            status = send_payload(self.requester, self.target, identity, payload)
        except (httpx.HTTPError, RequestFailure, DeliveryRejected) as error:
            self.save_failed(identity, payload, error)
        print(f"accepted submission_id={identity} status={status}", flush=True)

    def save_failed(self, identity: str, payload: bytes, error: Exception) -> NoReturn:
        with self.spool_factory() as spool:
            spool.enqueue_payload(identity, payload)
        reason = "network error"
        if isinstance(error, (RequestFailure, DeliveryRejected)):
            reason = str(error)
        raise DeliveryFailure(
            f"Submission {identity}: delivery failed ({reason}); saved locally; retry with deliver"
        ) from error
