"""Count and byte bounds with immutable submission identities."""

from collections.abc import Iterable, Iterator
from dataclasses import dataclass, field
from datetime import UTC, datetime
from uuid import UUID, uuid4

from .. import __version__
from ..models import Collector, Observation, Submission
from ..models.validation import encode_document

MAX_BYTES = 1_048_576
MAX_OBSERVATIONS = 100


@dataclass
class BatchMetadata:
    scraper_id: str
    shop_id: str
    run_id: UUID
    identity: UUID = field(default_factory=uuid4)
    created_at: datetime = field(default_factory=lambda: datetime.now(UTC))

    def build(self, observations: list[Observation]) -> Submission:
        return Submission(
            schema_version="1.0",
            submission_id=self.identity,
            scraper_id=self.scraper_id,
            shop_id=self.shop_id,
            run_id=self.run_id,
            created_at=self.created_at,
            collector=Collector(name="super-muffato", version=__version__),
            observations=observations,
        )

    def renew(self) -> None:
        self.identity, self.created_at = uuid4(), datetime.now(UTC)


class BatchBuilder:
    def __init__(self, metadata: BatchMetadata):
        self.metadata = metadata
        self.observations = []
        self.encoded_size = 0

    def append(self, observation: Observation) -> Submission | None:
        completed = None
        size = len(encode_document(observation.model_dump(mode="json")))
        if self.full_after(size):
            completed = self.finish()
            self.reset()
        self.add(observation, size)
        return completed

    def full_after(self, size: int) -> bool:
        if not self.observations:
            return False
        return (
            len(self.observations) == MAX_OBSERVATIONS or self.encoded_size + 1 + size > MAX_BYTES
        )

    def add(self, observation: Observation, size: int) -> None:
        self.encoded_size += 1 + size
        if not self.observations:
            submission = self.metadata.build([observation])
            self.encoded_size = len(submission.encode())
        validate_batch_size(self.encoded_size)
        self.observations.append(observation)

    def finish(self) -> Submission | None:
        if self.observations:
            return self.metadata.build(self.observations)
        return None

    def reset(self) -> None:
        self.observations, self.encoded_size = [], 0
        self.metadata.renew()


def validate_batch_size(size: int) -> None:
    if size > MAX_BYTES:
        raise ValueError("A single observation exceeds the request byte limit")


def emit_submission(submission: Submission | None) -> Iterator[Submission]:
    if submission is not None:
        yield submission


def batches(
    observations: Iterable[Observation], *, scraper_id: str, shop_id: str, run_id: UUID
) -> Iterator[Submission]:
    builder = BatchBuilder(BatchMetadata(scraper_id, shop_id, run_id))
    for observation in observations:
        yield from emit_submission(builder.append(observation))
    yield from emit_submission(builder.finish())
