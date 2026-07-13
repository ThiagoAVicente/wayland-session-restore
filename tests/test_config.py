import re
from pathlib import Path

from hsr.config import Config, load_config


def test_defaults_when_no_file(tmp_path):
    cfg = load_config(tmp_path / "nope.toml")
    assert cfg.exclude == []
    assert cfg.launch_overrides == {}
    assert cfg.interval == 60


def test_load_full_config(tmp_path):
    p = tmp_path / "config.toml"
    p.write_text(
        'exclude = ["^Rofi$", "steam"]\n'
        "[launch_overrides]\n"
        'firefox = ["firefox"]\n'
        "[snapshot]\n"
        "interval = 30\n"
    )
    cfg = load_config(p)
    assert [r.pattern for r in cfg.exclude] == ["^Rofi$", "steam"]
    assert cfg.launch_overrides == {"firefox": ["firefox"]}
    assert cfg.interval == 30


def test_is_excluded_case_insensitive(tmp_path):
    p = tmp_path / "config.toml"
    p.write_text('exclude = ["^rofi$"]\n')
    cfg = load_config(p)
    assert cfg.is_excluded("Rofi")
    assert not cfg.is_excluded("firefox")


def test_default_config_path_respects_xdg(monkeypatch, tmp_path):
    from hsr.config import default_config_path

    monkeypatch.setenv("XDG_CONFIG_HOME", str(tmp_path))
    assert default_config_path() == tmp_path / "hyprland-session-restore" / "config.toml"
