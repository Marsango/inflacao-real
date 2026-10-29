"""Executable wire-contract checks; no scraper or core implementation required."""

import copy
import hashlib
import json
from pathlib import Path
import unittest

from jsonschema import Draft202012Validator, FormatChecker
import rfc8785


DIRECTORY = Path(__file__).resolve().parent
SCHEMA = json.loads((DIRECTORY / "schema.json").read_text(encoding="utf-8"))


def example(name):
    return json.loads((DIRECTORY / "examples" / name).read_text(encoding="utf-8"))


def validator():
    return Draft202012Validator(SCHEMA, format_checker=FormatChecker())


class ContractTests(unittest.TestCase):
    def test_schema_is_valid(self):
        Draft202012Validator.check_schema(SCHEMA)

    def test_all_examples(self):
        for path in sorted((DIRECTORY / "examples").glob("*.json")):
            with self.subTest(name=path.name):
                validator().validate(example(path.name))

    def test_invalid_price_representations(self):
        for amount in (24.9, 2490, "24,90", "24.9", "024.90", "-1.00", "NaN", "1e2", "24.90\n", None):
            with self.subTest(amount=amount):
                body = example("submission.json")
                body["observations"][0]["offers"][0]["amount"] = amount
                self.assertFalse(validator().is_valid(body))

    def test_unknown_fields_are_rejected_at_boundaries(self):
        for target in ("envelope", "observation", "product", "conditions"):
            body = example("submission.json")
            observation = body["observations"][0]
            node = {
                "envelope": body,
                "observation": observation,
                "product": observation["product"],
                "conditions": observation["offers"][0]["conditions"],
            }[target]
            node["unexpected"] = "value"
            with self.subTest(target=target):
                self.assertFalse(validator().is_valid(body))

    def test_utc_and_calendar_validation(self):
        for timestamp in (
            "2026-10-27T12:00:00", "2026-10-27T09:00:00-03:00",
            "2026-02-30T12:00:00Z", "2026-10-27T12:00:00.1234567Z",
        ):
            body = example("submission.json")
            body["observations"][0]["observed_at"] = timestamp
            with self.subTest(timestamp=timestamp):
                self.assertFalse(validator().is_valid(body))

    def test_batch_count_limits(self):
        body = example("submission.json")
        observation = body["observations"][0]
        for count in (0, 101):
            body["observations"] = [copy.deepcopy(observation) for _ in range(count)]
            with self.subTest(count=count):
                self.assertFalse(validator().is_valid(body))

    def test_missing_price_is_empty_offers_not_zero(self):
        body = example("unavailable.json")
        validator().validate(body)
        body["observations"][0]["offers"] = None
        self.assertFalse(validator().is_valid(body))

    def test_positive_quantities(self):
        body = example("submission.json")
        for quantity in ("0", "0.0", "1.0", "01", "-5", "5,5", "5\n", 5):
            body["observations"][0]["product"]["package"]["quantity"] = quantity
            with self.subTest(quantity=quantity):
                self.assertFalse(validator().is_valid(body))
        for quantity in ("0.000001", "0.5", "5", "350", "1.25"):
            body["observations"][0]["product"]["package"]["quantity"] = quantity
            with self.subTest(quantity=quantity):
                validator().validate(body)

    def test_installments_require_credit_card(self):
        body = example("submission.json")
        conditions = body["observations"][0]["offers"][0]["conditions"]
        conditions["installments"] = 3
        self.assertFalse(validator().is_valid(body))
        conditions["payment_method"] = "credit_card"
        validator().validate(body)

    def test_membership_program_cannot_contradict_requirement(self):
        body = example("submission.json")
        body["observations"][0]["offers"][0]["conditions"]["membership_program"] = "Clube"
        self.assertFalse(validator().is_valid(body))

    def test_invalid_ids_and_location_codes(self):
        for field, value in (
            ("scraper_id", "collector-sp-01\n"),
            ("submission_id", "not-a-uuid"),
            ("gtin", "12345678\n"),
            ("postal_code", "01001000\n"),
            ("state", "XX"),
        ):
            body = example("submission.json")
            observation = body["observations"][0]
            if field == "gtin":
                observation["product"][field] = value
            elif field in ("postal_code", "state"):
                observation["context"][field] = value
            else:
                body[field] = value
            with self.subTest(field=field):
                self.assertFalse(validator().is_valid(body))

    def test_canonicalization_ignores_wire_formatting(self):
        body = example("submission.json")
        reordered = {key: body[key] for key in reversed(body)}
        escaped = json.loads(json.dumps(reordered, ensure_ascii=True, indent=4))
        self.assertEqual(rfc8785.dumps(body), rfc8785.dumps(escaped))

    def test_fingerprint_changes_for_changed_content(self):
        body = example("submission.json")
        original = hashlib.sha256(rfc8785.dumps(body)).hexdigest()
        body["observations"][0]["offers"][0]["amount"] = "25.90"
        self.assertNotEqual(original, hashlib.sha256(rfc8785.dumps(body)).hexdigest())

    def test_observation_fingerprint_is_independent_of_envelope(self):
        body = example("submission.json")
        original = rfc8785.dumps({"shop_id": body["shop_id"], "observation": body["observations"][0]})
        body["submission_id"] = "8d8d6e4c-0a9a-4b53-b688-ac2c2758317a"
        body["run_id"] = "f2672177-ae56-4bba-8f79-c9db8c2cf540"
        rebatched = rfc8785.dumps({"shop_id": body["shop_id"], "observation": body["observations"][0]})
        self.assertEqual(original, rebatched)
        body["shop_id"] = "another-shop"
        self.assertNotEqual(original, rfc8785.dumps({"shop_id": body["shop_id"], "observation": body["observations"][0]}))

    def test_shared_fingerprint_vectors(self):
        vectors = json.loads((DIRECTORY / "fingerprints.json").read_text(encoding="utf-8"))
        body = json.loads((DIRECTORY / vectors["example"]).read_text(encoding="utf-8"))
        observation = {"shop_id": body["shop_id"], "observation": body["observations"][0]}
        self.assertEqual(vectors["submission_sha256"], hashlib.sha256(rfc8785.dumps(body)).hexdigest())
        self.assertEqual(vectors["first_observation_sha256"], hashlib.sha256(rfc8785.dumps(observation)).hexdigest())

    def test_offer_order_is_part_of_identity(self):
        body = example("submission.json")
        original = rfc8785.dumps(body)
        body["observations"][0]["offers"].reverse()
        self.assertNotEqual(original, rfc8785.dumps(body))


if __name__ == "__main__":
    unittest.main()
