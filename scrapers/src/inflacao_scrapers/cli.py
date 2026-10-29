"""Parse command-line arguments and dispatch one focused command."""

import argparse
import sys
from pathlib import Path

from .runner import run


def argument_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--config", type=Path, default=Path("config.toml"), help="Scraper or runner TOML"
    )
    commands = parser.add_subparsers(dest="command", required=True)
    add_discovery_arguments(commands)
    commands.add_parser(
        "collect", help="Discover targets and deliver prices; save failed batches locally"
    )
    commands.add_parser("deliver", help="Send pending batches with INGESTION_TOKEN")
    commands.add_parser("status", help="Show local spool backlog")
    return parser


def add_discovery_arguments(commands) -> None:
    discover = commands.add_parser(
        "discover", help="Inspect products without queueing observations"
    )
    discover.add_argument("--query", required=True)
    discover.add_argument("--limit", type=int, choices=range(1, 51), default=20, metavar="1..50")


def main() -> int:
    parser = argument_parser()
    arguments = parser.parse_args()
    try:
        return run(arguments)
    except (ValueError, RuntimeError, OSError) as error:
        print(f"error: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
