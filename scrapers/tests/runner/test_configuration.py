from pathlib import Path

import pytest

from inflacao_scrapers.config import load_configurations


def write_config(tmp_path, content):
    path = tmp_path / "config.toml"
    path.write_text(content)
    return path


def test_disabled_unregistered_adapter_does_not_require_configuration(run_scrapers, capsys):
    execution = run_scrapers('[[scrapers]]\nadapter="not-installed"\nenabled=false\n')
    assert execution.status == 0
    assert not execution.requests
    assert not execution.clients
    assert "scrapers=0 failed=0 complete=True" in capsys.readouterr().out


def test_unknown_enabled_adapter_fails_before_network(run_scrapers, capsys):
    execution = run_scrapers('[[scrapers]]\nadapter="not-installed"\nenabled=true\n')
    assert execution.status == 1
    assert not execution.requests
    assert "Unknown adapter" in capsys.readouterr().err


@pytest.mark.parametrize("value", ['"false"', "0", '"true"'])
def test_enabled_flags_require_actual_booleans(tmp_path, value):
    path = write_config(tmp_path, f'[[scrapers]]\nadapter="super-muffato"\nenabled={value}\n')
    with pytest.raises(ValueError, match="enabled"):
        load_configurations(path)


def test_disabled_adapter_between_enabled_entries_is_skipped(run_scrapers):
    configuration = """
[[scrapers]]
scraper_id="first"
[[scrapers]]
adapter="not-installed"
enabled=false
[[scrapers]]
scraper_id="last"
"""
    execution = run_scrapers(configuration)
    assert execution.status == 0
    assert len(execution.clients) == 2


def test_each_adapter_validates_its_own_options(tmp_path, additional_adapter):
    path = write_config(
        tmp_path,
        """
[[scrapers]]
adapter="fixture-shop"
scraper_id="test"
shop_id="fixture-shop"
shop_url="https://fixture-shop.example"
catalog_path="/custom"
sales_channel="8"
""",
    )
    with pytest.raises(ValueError, match="sales_channel"):
        load_configurations(path)


def test_duplicate_spool_paths_are_rejected(tmp_path):
    path = write_config(
        tmp_path,
        """
[[scrapers]]
scraper_id="first"
spool_path="data/shared.sqlite3"
[[scrapers]]
scraper_id="second"
spool_path="data/../data/shared.sqlite3"
""",
    )
    with pytest.raises(ValueError, match="distinct spool_path"):
        load_configurations(path)


def test_legacy_config_retains_original_queue_location(tmp_path):
    path = write_config(tmp_path, 'scraper_id="existing"\n')
    settings = load_configurations(path)
    assert len(settings) == 1
    assert settings[0].adapter == "super-muffato"
    assert settings[0].spool_path == "data/spool.sqlite3"


def test_runner_defaults_isolate_queues_even_with_shared_scraper_identity(tmp_path):
    path = write_config(
        tmp_path,
        """
[[scrapers]]
scraper_id="shared"
shop_id="first-shop"
[[scrapers]]
scraper_id="shared"
shop_id="second-shop"
""",
    )
    settings = load_configurations(path)
    assert [setting.spool_path for setting in settings] == [
        "data/first-shop/shared/spool.sqlite3",
        "data/second-shop/shared/spool.sqlite3",
    ]


def test_duplicate_default_queue_locations_require_explicit_paths(tmp_path):
    path = write_config(
        tmp_path, '[[scrapers]]\nscraper_id="same"\n[[scrapers]]\nscraper_id="same"\n'
    )
    with pytest.raises(ValueError, match="distinct spool_path"):
        load_configurations(path)


@pytest.mark.parametrize("value", ['"../escape"', '""', '"shop/escape"'])
def test_shop_identity_cannot_escape_default_spool_directory(tmp_path, value):
    path = write_config(tmp_path, f'[[scrapers]]\nscraper_id="test"\nshop_id={value}\n')
    with pytest.raises(ValueError, match="shop_id"):
        load_configurations(path)


def test_runner_root_cannot_silently_drop_settings(tmp_path):
    path = write_config(
        tmp_path, 'core_url="https://core.example"\n[[scrapers]]\nscraper_id="test"\n'
    )
    with pytest.raises(ValueError, match="core_url"):
        load_configurations(path)


def test_runner_example_loads():
    path = Path(__file__).parents[2] / "scrapers.example.toml"
    settings = load_configurations(path)
    assert len(settings) == 3
    assert settings[0].sales_channel == "8"
    assert [setting.adapter for setting in settings] == ["super-muffato", "angeloni", "festval"]
