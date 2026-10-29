"""Atomic SQLite operations for the local spool, independent of application storage."""

import sqlite3
from pathlib import Path


def open_spool(path: Path) -> sqlite3.Connection:
    path.parent.mkdir(parents=True, exist_ok=True)
    connection = sqlite3.connect(path, timeout=10)
    connection.execute("PRAGMA synchronous=FULL")
    connection.execute("""CREATE TABLE IF NOT EXISTS batches (
        sequence INTEGER PRIMARY KEY,
        submission_id TEXT NOT NULL UNIQUE,
        payload BLOB NOT NULL
    )""")
    connection.commit()
    return connection


def store_payload(connection, identity: str, payload: bytes, capacity: int) -> None:
    with connection:
        connection.execute("BEGIN IMMEDIATE")
        reserve_payload(connection, identity, payload, capacity)


def reserve_payload(connection, identity: str, payload: bytes, capacity: int) -> None:
    cursor = connection.execute("SELECT payload FROM batches WHERE submission_id=?", (identity,))
    existing = cursor.fetchone()
    if existing:
        return require_unchanged_payload(existing[0], payload)
    require_capacity(connection, len(payload), capacity)
    connection.execute(
        "INSERT INTO batches(submission_id,payload) VALUES (?,?)", (identity, payload)
    )


def require_unchanged_payload(existing: bytes, payload: bytes) -> None:
    if existing != payload:
        raise ValueError("Cannot replace an immutable queued submission")


def require_capacity(connection, size: int, capacity: int) -> None:
    cursor = connection.execute("SELECT coalesce(sum(length(payload)),0) FROM batches")
    used = cursor.fetchone()[0]
    if used + size > capacity:
        raise RuntimeError("Delivery spool exhausted; deliver pending batches before collecting")
