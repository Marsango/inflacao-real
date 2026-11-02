"""Manual collection containers retain diagnostics, including after failure."""

import json
import os
import shutil
import subprocess
import unittest
from unittest.mock import Mock, call, patch

import lifecycle


class ManualCollectionLogsTests(unittest.TestCase):
    def test_successful_collection_keeps_logs_and_resumes_scheduler(self):
        verify_collection(False)

    def test_failed_collection_keeps_logs_and_resumes_scheduler(self):
        verify_collection(True)

    @unittest.skipUnless(shutil.which("docker"), "Docker Compose CLI required")
    def test_retained_manual_runs_do_not_inherit_scheduler_restart_policy(self):
        assert compose_configuration(False)["services"]["scrapers"]["restart"] == "unless-stopped"
        assert compose_configuration(True)["services"]["scrapers"]["restart"] == "no"


def verify_collection(failed):
    compose = Mock(side_effect=collection_outcomes(failed))
    with patch.object(lifecycle, "compose", compose):
        run_manual_collection(failed)
    arguments = compose.call_args_list[1].args
    assert arguments[:2] == ("--file", str(lifecycle.ROOT / "deployment/manual-collection.yaml"))
    assert arguments[2:5] == ("run", "--no-deps", "scrapers")
    assert "--rm" not in arguments
    assert compose.call_args_list[-1] == call("start", "scrapers")


def collection_outcomes(failed):
    if failed:
        return [None, subprocess.CalledProcessError(1, "collect"), None]
    return [None, None, None]


def run_manual_collection(failed):
    if failed:
        require_failed_collection()
        return
    lifecycle.scraper_command("collect")


def require_failed_collection():
    test = unittest.TestCase()
    with test.assertRaises(subprocess.CalledProcessError):
        lifecycle.scraper_command("collect")


def compose_configuration(manual):
    root = lifecycle.ROOT / "deployment"
    arguments = ["docker", "compose", "--file", str(root / "compose.yaml")]
    if manual:
        arguments.extend(["--file", str(root / "manual-collection.yaml")])
    environment = {**os.environ, "POSTGRES_PASSWORD": "test", "INGESTION_TOKEN": "test"}
    result = subprocess.run([*arguments, "config", "--format", "json"],
                            env=environment, capture_output=True, text=True, check=True)
    return json.loads(result.stdout)
