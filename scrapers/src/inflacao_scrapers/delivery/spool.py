"""Durable immutable submissions, acknowledged only after successful delivery."""

from collections.abc import Iterator
from pathlib import Path

from ..models import Submission
from .batching import MAX_BYTES
from .storage import open_spool, store_payload


class Spool:
    def __init__(self, path: Path, capacity: int = 64 * 1024 * 1024):
        self.capacity = capacity
        self.database = open_spool(path)

    def close(self):
        self.database.close()

    def __enter__(self):
        return self

    def __exit__(self, *_):
        self.close()

    def enqueue(self, submission: Submission) -> None:
        payload = submission.encode()
        if len(payload) > MAX_BYTES:
            raise ValueError("Batch exceeds ingestion byte limit")
        store_payload(self.database, str(submission.submission_id), payload, self.capacity)

    def pending(self) -> Iterator[tuple[str, bytes]]:
        yield from self.database.execute(
            "SELECT submission_id,payload FROM batches ORDER BY sequence"
        )

    def acknowledge(self, identity: str) -> None:
        with self.database:
            self.database.execute("DELETE FROM batches WHERE submission_id=?", (identity,))

    def status(self) -> tuple[int, int]:
        cursor = self.database.execute(
            "SELECT count(*),coalesce(sum(length(payload)),0) FROM batches"
        )
        return cursor.fetchone()
