"""Run enabled scrapers every day; defaults to 03:00 in São Paulo."""

from pathlib import Path

from inflacao_scrapers.scheduling import main

if __name__ == "__main__":
    raise SystemExit(main(default_config=Path(__file__).with_name("config.toml")))
