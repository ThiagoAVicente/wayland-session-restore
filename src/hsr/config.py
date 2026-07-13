import os
import re
import tomllib
from dataclasses import dataclass, field
from pathlib import Path


@dataclass
class Config:
    exclude: list[re.Pattern] = field(default_factory=list)
    launch_overrides: dict[str, list[str]] = field(default_factory=dict)
    interval: int = 60

    def is_excluded(self, window_class: str) -> bool:
        return any(r.search(window_class) for r in self.exclude)


def default_config_path() -> Path:
    base = Path(os.environ.get("XDG_CONFIG_HOME", Path.home() / ".config"))
    return base / "hyprland-session-restore" / "config.toml"


def load_config(path: Path | None = None) -> Config:
    path = path or default_config_path()
    if not path.is_file():
        return Config()
    data = tomllib.loads(path.read_text())
    return Config(
        exclude=[re.compile(p, re.IGNORECASE) for p in data.get("exclude", [])],
        launch_overrides={
            k: list(v) for k, v in data.get("launch_overrides", {}).items()
        },
        interval=int(data.get("snapshot", {}).get("interval", 60)),
    )
