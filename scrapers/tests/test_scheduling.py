import argparse
import sys
from datetime import datetime, time, timedelta
from types import SimpleNamespace
from zoneinfo import ZoneInfo

import pytest

from inflacao_scrapers import scheduling

SAO_PAULO = ZoneInfo("America/Sao_Paulo")


@pytest.fixture
def clock(monkeypatch):
    clock = SimpleNamespace(current=datetime(2026, 10, 29, 2, 59, tzinfo=SAO_PAULO))

    class ControlledDatetime(datetime):
        @classmethod
        def now(cls, zone):
            current = clock.current
            return current.astimezone(zone)

    monkeypatch.setattr(scheduling, "datetime", ControlledDatetime)
    monkeypatch.setattr(scheduling, "sleep", lambda seconds: advance_clock(clock, seconds))
    return clock


def advance_clock(clock, seconds):
    clock.current += timedelta(seconds=seconds)


@pytest.mark.parametrize(
    ("current", "expected"),
    [
        ("2026-10-29T02:59:00-03:00", "2026-10-29T03:00:00-03:00"),
        ("2026-10-29T03:00:00-03:00", "2026-10-30T03:00:00-03:00"),
        ("2026-10-29T15:00:00-03:00", "2026-10-30T03:00:00-03:00"),
        ("2026-12-31T23:00:00-03:00", "2027-01-01T03:00:00-03:00"),
    ],
)
def test_next_run_uses_the_next_wall_clock_time(current, expected):
    result = scheduling.next_run(datetime.fromisoformat(current), time(3))
    assert result == datetime.fromisoformat(expected)


def test_wait_rechecks_the_clock_at_most_every_minute(clock, monkeypatch):
    sleeps = []

    def sleep(seconds):
        sleeps.append(seconds)
        advance_clock(clock, seconds)

    monkeypatch.setattr(scheduling, "sleep", sleep)
    scheduled = clock.current + timedelta(seconds=130)
    scheduling.wait_until(scheduled)
    assert clock.current == scheduled
    assert sleeps == [60, 60, 10]


def test_wait_handles_a_forward_clock_jump(clock, monkeypatch):
    scheduled = clock.current + timedelta(hours=2)
    monkeypatch.setattr(scheduling, "sleep", lambda seconds: advance_clock(clock, 86400))
    scheduling.wait_until(scheduled)
    assert clock.current > scheduled


def daily_arguments(tmp_path):
    return argparse.Namespace(config=tmp_path / "config.toml", at=time(3), timezone=SAO_PAULO)


def stop_after_two_collections(clock, starts):
    starts.append(clock.current)
    if len(starts) == 2:
        raise KeyboardInterrupt


def test_failed_run_does_not_stop_next_days_collection(clock, monkeypatch, tmp_path, capsys):
    starts = []

    def collect(arguments):
        assert arguments.command == "collect"
        assert arguments.config == tmp_path / "config.toml"
        stop_after_two_collections(clock, starts)
        return 1

    monkeypatch.setattr(scheduling, "run", collect)
    with pytest.raises(KeyboardInterrupt):
        scheduling.run_daily(daily_arguments(tmp_path))
    assert starts == [datetime(2026, 10, day, 3, tzinfo=SAO_PAULO) for day in (29, 30)]
    assert "exit_status=1" in capsys.readouterr().out


def test_unexpected_failure_does_not_stop_next_days_collection(
    clock, monkeypatch, tmp_path, capsys
):
    starts = []

    def collect(arguments):
        stop_after_two_collections(clock, starts)
        raise TypeError("sensitive detail")

    monkeypatch.setattr(scheduling, "run", collect)
    with pytest.raises(KeyboardInterrupt):
        scheduling.run_daily(daily_arguments(tmp_path))
    assert len(starts) == 2
    output = capsys.readouterr()
    assert "Scheduled collection failed: TypeError" in output.err
    assert "sensitive detail" not in output.err


def test_long_collection_skips_missed_slots_and_never_overlaps(clock, monkeypatch, tmp_path):
    starts = []

    def collect(arguments):
        stop_after_two_collections(clock, starts)
        advance_clock(clock, 25 * 3600)
        return 0

    monkeypatch.setattr(scheduling, "run", collect)
    with pytest.raises(KeyboardInterrupt):
        scheduling.run_daily(daily_arguments(tmp_path))
    assert starts == [datetime(2026, 10, day, 3, tzinfo=SAO_PAULO) for day in (29, 31)]


def test_scheduler_uses_the_configured_timezone(clock, monkeypatch, tmp_path):
    arguments = daily_arguments(tmp_path)
    arguments.timezone = ZoneInfo("UTC")
    arguments.at = time(6)
    starts = []

    def collect(arguments):
        starts.append(clock.current)
        raise KeyboardInterrupt

    monkeypatch.setattr(scheduling, "run", collect)
    with pytest.raises(KeyboardInterrupt):
        scheduling.run_daily(arguments)
    assert starts == [datetime(2026, 10, 29, 3, tzinfo=SAO_PAULO)]


def test_startup_rejects_invalid_configuration_before_waiting(tmp_path, monkeypatch, capsys):
    config = tmp_path / "config.toml"
    config.write_text('[[scrapers]]\nadapter="unknown"\n')
    monkeypatch.setattr(sys, "argv", ["schedule", "--config", str(config)])
    assert scheduling.main() == 1
    assert "Unknown adapter" in capsys.readouterr().err


def test_main_stops_cleanly_on_keyboard_interrupt(tmp_path, monkeypatch, capsys):
    config = tmp_path / "config.toml"
    config.write_text("[[scrapers]]\nenabled=false\n")
    monkeypatch.setattr(sys, "argv", ["schedule"])

    def interrupt(arguments):
        raise KeyboardInterrupt

    monkeypatch.setattr(scheduling, "run_daily", interrupt)
    assert scheduling.main(default_config=config) == 0
    assert "Scheduler stopped" in capsys.readouterr().out


def test_default_schedule(tmp_path):
    parser = scheduling.argument_parser(tmp_path / "config.toml")
    arguments = parser.parse_args([])
    assert arguments.at == time(3)
    assert arguments.timezone == SAO_PAULO
    assert arguments.config == tmp_path / "config.toml"


@pytest.mark.parametrize("value", ["24:00", "12:60", "3:00", "03:00:00", "invalid"])
def test_invalid_daily_time_is_rejected(value):
    with pytest.raises(argparse.ArgumentTypeError, match="HH:MM"):
        scheduling.daily_time(value)


@pytest.mark.parametrize("value", ["00:00", "03:00", "23:59"])
def test_valid_daily_time(value):
    assert scheduling.daily_time(value) == time.fromisoformat(value)


def test_unknown_timezone_is_rejected():
    with pytest.raises(argparse.ArgumentTypeError, match="Unknown timezone"):
        scheduling.timezone("Not/A_Timezone")
