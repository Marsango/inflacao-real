"""Shared-schema validation and JSON-safe serialization."""

import json
from datetime import UTC
from importlib.resources import files
from urllib.parse import urlsplit

from jsonschema import Draft202012Validator, FormatChecker
from jsonschema.exceptions import ValidationError as SchemaValidationError

SCHEMA = json.loads(files("inflacao_scrapers").joinpath("ingestion_v1.json").read_text())


def encode_document(document: dict) -> bytes:
    encoded = json.dumps(document, ensure_ascii=False, allow_nan=False, separators=(",", ":"))
    return encoded.encode("utf-8")


def validate_wire(document: dict, definition: str | None = None) -> None:
    schema = schema_definition(definition)
    validate_schema(document, schema)
    encode_document(document)
    reject_null_characters(document)


def schema_definition(definition: str | None) -> dict:
    if definition is None:
        return SCHEMA
    return {"$defs": SCHEMA["$defs"], "$ref": f"#/$defs/{definition}"}


def validate_schema(document: dict, schema: dict) -> None:
    validator = Draft202012Validator(schema, format_checker=FormatChecker())
    try:
        validator.validate(document)
    except SchemaValidationError as error:
        path = ".".join(map(str, error.absolute_path))
        raise ValueError(f"Ingestion contract violation: {path} ({error.validator})") from error


def reject_null_characters(value) -> None:
    if isinstance(value, str) and "\x00" in value:
        raise ValueError("NUL is not valid ingestion data")
    if isinstance(value, dict):
        return reject_null_values(value.values())
    if isinstance(value, list):
        reject_null_values(value)


def reject_null_values(values) -> None:
    for value in values:
        reject_null_characters(value)


def validate_url(value: str) -> str:
    parsed = urlsplit(value)
    if parsed.scheme not in {"http", "https"} or not parsed.hostname:
        raise ValueError("Expected an absolute HTTP(S) URL without credentials")
    if parsed.username is not None or parsed.password is not None:
        raise ValueError("Expected an absolute HTTP(S) URL without credentials")
    if any(character.isspace() for character in value):
        raise ValueError("Expected an absolute HTTP(S) URL without credentials")
    return value


def utc_time(value) -> str:
    value = value.astimezone(UTC)
    timestamp = value.isoformat()
    return timestamp.replace("+00:00", "Z")
