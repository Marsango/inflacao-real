"""Decode exact source numbers without silently replacing duplicate keys."""

import json
from decimal import Decimal


def object_pairs(pairs):
    result = {}
    for key, value in pairs:
        add_unique_pair(result, key, value)
    return result


def add_unique_pair(result: dict, key: str, value) -> None:
    if key in result:
        raise ValueError("Duplicate source JSON key")
    result[key] = value


def reject_nonfinite(_):
    raise ValueError("Nonfinite source JSON number")


def decode_response(raw: bytes):
    return json.loads(
        raw, parse_float=Decimal, parse_constant=reject_nonfinite, object_pairs_hook=object_pairs
    )
