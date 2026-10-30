"""Small production CLI. Run via the repository's deploy entry point."""
import argparse
import fcntl
import os
import sys
from pathlib import Path
import subprocess
from runtime import compose
from lifecycle import start, scraper_command
from archive import backup, restore


def dispatch(arguments):
    actions = {"up": start, "status": lambda: compose("ps"),
               "logs": lambda: compose("logs", "--tail", "100", "-f"),
               "collect": lambda: scraper_command("collect"),
               "deliver": lambda: scraper_command("deliver"),
               "backup": lambda: backup(Path(arguments.file)),
               "restore": lambda: restore(Path(arguments.file))}
    actions[arguments.action]()


def parse_arguments():
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="action", required=True)
    for action in ["up", "status", "logs", "collect", "deliver"]:
        subparsers.add_parser(action)
    for action in ["backup", "restore"]:
        subparsers.add_parser(action).add_argument("file")
    return parser.parse_args()


def main():
    os.umask(0o077)
    arguments = parse_arguments()
    if arguments.action in {"status", "logs"}:
        dispatch(arguments)
        return
    with open("/run/lock/nossa-inflacao-deploy.lock", "w") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        dispatch(arguments)


if __name__ == "__main__":
    try:
        main()
    except (OSError, RuntimeError, subprocess.CalledProcessError) as error:
        print(f"Deployment failed: {error}", file=sys.stderr)
        sys.exit(1)
