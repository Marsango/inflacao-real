"""Keep the shared collection runner on a daily wall-clock schedule."""

import argparse
import re
import sys
from datetime import datetime, time, timedelta
from pathlib import Path
from time import sleep
from zoneinfo import ZoneInfo, ZoneInfoNotFoundError

from .config import load_configurations
from .runner import run


def daily_time(value: str) -> time:
    if not re.fullmatch(r"(?:[01][0-9]|2[0-3]):[0-5][0-9]", value):
        raise argparse.ArgumentTypeError("Use a 24-hour time in HH:MM format")
    return time.fromisoformat(value)


def timezone(value: str) -> ZoneInfo:
    try:
        return ZoneInfo(value)
    except ZoneInfoNotFoundError as error:
        raise argparse.ArgumentTypeError(f"Unknown timezone: {value}") from error


def argument_parser(default_config: Path) -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--config", type=Path, default=default_config, help="Scraper or runner TOML"
    )
    parser.add_argument(
        "--at", type=daily_time, default="03:00", help="Daily start time, HH:MM (default: 03:00)"
    )
    parser.add_argument(
        "--timezone",
        type=timezone,
        default="America/Sao_Paulo",
        help="IANA timezone (default: America/Sao_Paulo)",
    )
    return parser


def next_run(current: datetime, start_time: time) -> datetime:
    scheduled = datetime.combine(current.date(), start_time, tzinfo=current.tzinfo)
    if scheduled <= current:
        return scheduled + timedelta(days=1)
    return scheduled


def wait_until(scheduled: datetime) -> None:
    remaining = seconds_until(scheduled)
    while remaining > 0:
        sleep(min(remaining, 60))
        remaining = seconds_until(scheduled)


def seconds_until(scheduled: datetime) -> float:
    current = datetime.now(scheduled.tzinfo)
    return scheduled.timestamp() - current.timestamp()


def collect_once(config: Path) -> None:
    print("Starting scheduled collection", flush=True)
    try:
        result = run(argparse.Namespace(config=config, command="collect"))
        print(f"Scheduled collection finished: exit_status={result}", flush=True)
    except Exception as error:
        error_name = type(error).__name__
        print(f"Scheduled collection failed: {error_name}", file=sys.stderr, flush=True)


def run_daily(arguments) -> None:
    while True:
        current = datetime.now(arguments.timezone)
        scheduled = next_run(current, arguments.at)
        scheduled_time = scheduled.isoformat()
        print(f"Next collection: {scheduled_time} ({arguments.timezone})", flush=True)
        wait_until(scheduled)
        collect_once(arguments.config)


def main(default_config: Path = Path("config.toml")) -> int:
    parser = argument_parser(default_config)
    arguments = parser.parse_args()
    try:
        load_configurations(arguments.config)
        run_daily(arguments)
    except KeyboardInterrupt:
        print("Scheduler stopped", flush=True)
        return 0
    except (ValueError, RuntimeError, OSError) as error:
        print(f"error: {error}", file=sys.stderr)
        return 1
    return 0
