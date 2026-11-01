"""Existing VPS upgrades must receive new collectors without losing local settings."""
import tempfile
import tomllib
import unittest
from pathlib import Path
from unittest.mock import Mock, patch

import lifecycle
import settings

EXISTING = '''# Custom VPS configuration
[[scrapers]]
adapter = "super-muffato"
scraper_id = "muffato-curitiba-01"
spool_path = "data/original.sqlite3"
request_interval_seconds = 2.0

[[scrapers]]
adapter = "pao-de-acucar"
enabled = false
scraper_id = "muffato-curitiba-01"
spool_path = "data/custom-pao.sqlite3"
'''


def synchronize(directory, original):
    path = Path(directory) / "scrapers.toml"
    path.write_text(original)
    with patch.object(settings, "CONFIG", Path(directory)):
        settings.synchronize_scrapers()
    return path.read_text()


class ConfigurationUpgradeTests(unittest.TestCase):
    def test_upgrade_adds_missing_collectors_and_preserves_custom_entries(self):
        with tempfile.TemporaryDirectory() as directory:
            updated = synchronize(directory, EXISTING)
        self.assertTrue(updated.startswith(EXISTING.rstrip()))
        entries = tomllib.loads(updated)["scrapers"]
        self.assertEqual(len(entries), 8)
        self.assertEqual(entries[0]["spool_path"], "data/original.sqlite3")
        self.assertEqual(entries[0]["request_interval_seconds"], 2.0)
        self.assertFalse(entries[1]["enabled"])
        self.assertEqual(entries[1]["spool_path"], "data/custom-pao.sqlite3")
        self.assertEqual({entry["adapter"] for entry in entries}, {
            "super-muffato", "angeloni", "festval", "pao-de-acucar",
            "mambo", "hirota", "sonda", "nagumo"})

    def test_repeated_upgrade_does_not_duplicate_collectors(self):
        with tempfile.TemporaryDirectory() as directory:
            first = synchronize(directory, EXISTING)
            second = synchronize(directory, first)
        self.assertEqual(first, second)

    def test_fresh_install_keeps_template_unchanged(self):
        template = (settings.ROOT / "scrapers/scrapers.example.toml").read_text()
        with tempfile.TemporaryDirectory() as directory:
            updated = synchronize(directory, template)
        self.assertEqual(updated, template)

    def test_invalid_existing_configuration_is_not_replaced(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "scrapers.toml"
            self.assertRaises(ValueError, synchronize, directory, "scrapers = [")
            self.assertEqual(path.read_text(), "scrapers = [")



def deployment_mocks(directory, events):
    return {
        "CONFIG": directory, "bootstrap": Mock(), "initialize": Mock(),
        "save_domain": Mock(), "require_clean_checkout": Mock(),
        "revision": Mock(return_value="test-revision"),
        "compose": Mock(side_effect=lambda *arguments: events.append(arguments)),
        "synchronize_scrapers": Mock(side_effect=lambda: events.append("synchronize")),
        "start_application": Mock(side_effect=lambda: events.append("start")),
    }


class DeploymentWorkflowTests(unittest.TestCase):
    def test_upgrade_synchronizes_before_recreating_scheduler(self):
        events = []
        with tempfile.TemporaryDirectory() as directory:
            self.run_upgrade(Path(directory), events)
        self.assertEqual(events, [
            ("build", "--pull"), ("stop", "scrapers"), "synchronize", "start"])

    def run_upgrade(self, directory, events):
        with patch.multiple(lifecycle, **deployment_mocks(directory, events)):
            lifecycle.start()
        self.assertEqual((directory / "revision").read_text(), "test-revision\n")
