"""Run each enabled scraper in isolation and report an aggregate exit status."""

import sqlite3
import sys

import httpx

from .adapters.registry import registered_adapter
from .commands import COMMANDS
from .config import load_configurations


def run(arguments) -> int:
    configurations = load_configurations(arguments.config)
    results = [run_scraper(settings, arguments) for settings in configurations]
    failures = sum(result != 0 for result in results)
    print(f"scrapers={len(results)} failed={failures} complete={failures == 0}", flush=True)
    return int(failures != 0)


def run_scraper(settings, arguments) -> int:
    print(
        f"scraper_id={settings.scraper_id} shop_id={settings.shop_id} "
        f"adapter={settings.adapter} command={arguments.command}",
        flush=True,
    )
    try:
        adapter = registered_adapter(settings.adapter)
        command = COMMANDS[arguments.command]
        return command(settings, arguments, adapter)
    except httpx.HTTPError:
        return report_failure(settings, "HTTP request failed")
    except (ValueError, RuntimeError, OSError, sqlite3.Error) as error:
        return report_failure(settings, str(error))
    except Exception as error:
        error_name = type(error).__name__
        return report_failure(settings, f"Unexpected {error_name} during scraper execution")


def report_failure(settings, message: str) -> int:
    print(
        f"scraper_id={settings.scraper_id} adapter={settings.adapter}: {message}", file=sys.stderr
    )
    return 1
