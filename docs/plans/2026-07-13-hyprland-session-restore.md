# Hyprland Session Restore Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** CLI tool that periodically snapshots the Hyprland session (windows, workspaces, terminal cwds, launch commands) and restores it on login.

**Architecture:** Python package `hsr` with thin modules: `hypr.py` (hyprctl JSON wrapper), `proc.py` (/proc cmdline + deepest-descendant cwd), `config.py` (TOML config with exclude rules), `snapshot.py` (capture -> JSON state file), `restore.py` (state -> `hyprctl dispatch exec` with workspace/float rules), `cli.py` (argparse: snapshot/restore/watch). Terminals get their cwd back because restore wraps commands in `sh -c 'cd CWD && exec CMD'` and terminals spawn their shell in their own cwd.

**Tech Stack:** Python >=3.11, stdlib only (`tomllib`, `subprocess`, `json`, `argparse`). Tests: pytest. Packaging: `pyproject.toml` (hatchling), console scripts `hyprland-session-restore` and `hsr`.

**Non-goals (YAGNI):** other compositors, D-Bus, saving window content, re-tiling exact layout beyond workspace/floating geometry, socket-event watching (periodic snapshot suffices).

---

## State file schema

`~/.local/state/hyprland-session-restore/session.json` (respects `$XDG_STATE_HOME`):

```json
{
  "version": 1,
  "clients": [
    {
      "class": "foot",
      "title": "foot",
      "workspace": 1,
      "floating": false,
      "at": [21, 625],
      "size": [928, 554],
      "pinned": false,
      "fullscreen": 0,
      "cmdline": ["foot"],
      "cwd": "/home/vcnt/Projects/ss"
    }
  ]
}
```

## Config file

`~/.config/hyprland-session-restore/config.toml` (respects `$XDG_CONFIG_HOME`). All keys optional:

```toml
# Regexes matched (re.search, case-insensitive) against window class.
# Matching windows are neither snapshotted nor restored.
exclude = ["^Rofi$", "^steam_", "polkit"]

# Override the restore command for a class (list = argv).
# Default is the captured cmdline.
[launch_overrides]
firefox = ["firefox"]

[snapshot]
interval = 60   # seconds, used by `hsr watch`
```

Exclusion is applied at BOTH snapshot and restore time (so an updated config also filters old state files).

---

### Task 1: Scaffold — package, pyproject, LICENSE, README stub

**Files:**
- Create: `pyproject.toml`
- Create: `src/hsr/__init__.py`
- Create: `tests/__init__.py` (empty)
- Create: `LICENSE` (MIT, copyright 2026 Thiago Vicente)
- Create: `README.md` (stub)
- Create: `.gitignore`

- [ ] **Step 1: Write files**

`pyproject.toml`:
```toml
[build-system]
requires = ["hatchling"]
build-backend = "hatchling.build"

[project]
name = "hyprland-session-restore"
version = "0.1.0"
description = "Snapshot and restore Hyprland sessions: windows, workspaces, terminal cwds"
readme = "README.md"
license = { text = "MIT" }
requires-python = ">=3.11"

[project.scripts]
hyprland-session-restore = "hsr.cli:main"
hsr = "hsr.cli:main"

[tool.hatch.build.targets.wheel]
packages = ["src/hsr"]

[tool.pytest.ini_options]
testpaths = ["tests"]
pythonpath = ["src"]
```

`src/hsr/__init__.py`:
```python
__version__ = "0.1.0"
```

`.gitignore`:
```
__pycache__/
*.egg-info/
dist/
.pytest_cache/
.venv/
```

`README.md` stub:
```markdown
# wayland-session-restore

![vibecoded](https://img.shields.io/badge/vibecoded-100%25-ff69b4?style=for-the-badge)

Session restore for Hyprland. X11 had session management; Wayland killed it and nothing replaced it. This tool snapshots your session periodically via Hyprland IPC (clients, workspaces, terminal cwds via /proc, launch commands) and relaunches + repositions everything on startup.

WIP — see docs/plans/.
```

`LICENSE`: standard MIT text, `Copyright (c) 2026 Thiago Vicente`.

- [ ] **Step 2: Verify pytest collects (0 tests, no error)**

Run: `python3 -m pytest`
Expected: `no tests ran` (exit 5 is fine), no import/config errors.

- [ ] **Step 3: Commit**

```bash
git add -A && git commit -m "chore: scaffold package, pyproject, license, readme stub"
```

---

### Task 2: `config.py` — load TOML config with defaults

**Files:**
- Create: `src/hsr/config.py`
- Test: `tests/test_config.py`

- [ ] **Step 1: Write the failing tests**

`tests/test_config.py`:
```python
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
```

- [ ] **Step 2: Run tests, verify they fail**

Run: `python3 -m pytest tests/test_config.py -v`
Expected: FAIL — `ModuleNotFoundError: No module named 'hsr.config'`

- [ ] **Step 3: Implement**

`src/hsr/config.py`:
```python
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
```

- [ ] **Step 4: Run tests, verify pass**

Run: `python3 -m pytest tests/test_config.py -v` — Expected: 4 passed

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat: TOML config with exclude regexes and launch overrides"
```

---

### Task 3: `proc.py` — cmdline and deepest-descendant cwd from /proc

**Files:**
- Create: `src/hsr/proc.py`
- Test: `tests/test_proc.py`

Functions take a `proc_root: Path` argument (default `Path("/proc")`) so tests build a fake /proc in tmp_path.

- [ ] **Step 1: Write the failing tests**

`tests/test_proc.py`:
```python
from pathlib import Path

from hsr.proc import cmdline, deepest_cwd


def make_proc(root: Path, pid: int, ppid: int, cmd: list[str], cwd: Path):
    d = root / str(pid)
    d.mkdir()
    (d / "cmdline").write_bytes(b"\x00".join(c.encode() for c in cmd) + b"\x00")
    (d / "stat").write_text(f"{pid} ({cmd[0][:15]}) S {ppid} 0 0")
    cwd.mkdir(parents=True, exist_ok=True)
    (d / "cwd").symlink_to(cwd)


def test_cmdline(tmp_path):
    make_proc(tmp_path, 100, 1, ["foot", "--app-id=x"], tmp_path / "home")
    assert cmdline(100, proc_root=tmp_path) == ["foot", "--app-id=x"]


def test_cmdline_missing_pid(tmp_path):
    assert cmdline(999, proc_root=tmp_path) == []


def test_deepest_cwd_follows_child_chain(tmp_path):
    # foot(100) -> zsh(200) -> nvim(300); nvim cwd is the answer
    make_proc(tmp_path, 100, 1, ["foot"], tmp_path / "a")
    make_proc(tmp_path, 200, 100, ["zsh"], tmp_path / "b")
    make_proc(tmp_path, 300, 200, ["nvim"], tmp_path / "b" / "proj")
    assert deepest_cwd(100, proc_root=tmp_path) == str(tmp_path / "b" / "proj")


def test_deepest_cwd_no_children_returns_own(tmp_path):
    make_proc(tmp_path, 100, 1, ["firefox"], tmp_path / "home")
    assert deepest_cwd(100, proc_root=tmp_path) == str(tmp_path / "home")


def test_deepest_cwd_unreadable_returns_none(tmp_path):
    assert deepest_cwd(999, proc_root=tmp_path) is None
```

- [ ] **Step 2: Run tests, verify fail**

Run: `python3 -m pytest tests/test_proc.py -v`
Expected: FAIL — `ModuleNotFoundError: No module named 'hsr.proc'`

- [ ] **Step 3: Implement**

`src/hsr/proc.py`:
```python
from pathlib import Path

PROC = Path("/proc")


def cmdline(pid: int, proc_root: Path = PROC) -> list[str]:
    try:
        raw = (proc_root / str(pid) / "cmdline").read_bytes()
    except OSError:
        return []
    return [p.decode(errors="replace") for p in raw.split(b"\x00") if p]


def _ppid_map(proc_root: Path) -> dict[int, int]:
    """pid -> ppid for all readable processes."""
    out: dict[int, int] = {}
    for d in proc_root.iterdir():
        if not d.name.isdigit():
            continue
        try:
            stat = (d / "stat").read_text()
        except OSError:
            continue
        # ppid is field 4; comm (field 2) may contain spaces, so split after ')'
        after = stat.rsplit(")", 1)[-1].split()
        if len(after) >= 2:
            out[int(d.name)] = int(after[1])
    return out


def _cwd(pid: int, proc_root: Path) -> str | None:
    try:
        return str((proc_root / str(pid) / "cwd").resolve())
    except OSError:
        return None


def deepest_cwd(pid: int, proc_root: Path = PROC) -> str | None:
    """cwd of the deepest descendant of pid, falling back up the chain.

    Terminals keep their launch cwd; the user's shell/editor underneath holds
    the cwd worth restoring.
    """
    if not (proc_root / str(pid)).is_dir():
        return None
    ppids = _ppid_map(proc_root)
    children: dict[int, list[int]] = {}
    for p, pp in ppids.items():
        children.setdefault(pp, []).append(p)

    current, result = pid, _cwd(pid, proc_root)
    while children.get(current):
        current = max(children[current])  # newest child on ties
        result = _cwd(current, proc_root) or result
    return result
```

- [ ] **Step 4: Run tests, verify pass**

Run: `python3 -m pytest tests/test_proc.py -v` — Expected: 5 passed

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat: /proc introspection - cmdline and deepest-descendant cwd"
```

---

### Task 4: `hypr.py` — hyprctl wrapper

**Files:**
- Create: `src/hsr/hypr.py`
- Test: `tests/test_hypr.py`

- [ ] **Step 1: Write the failing tests**

`tests/test_hypr.py`:
```python
import json
from unittest.mock import patch, MagicMock

from hsr.hypr import clients, dispatch_exec


FAKE_CLIENTS = [
    {"class": "foot", "pid": 123, "mapped": True, "workspace": {"id": 1}},
    {"class": "bad", "pid": -1, "mapped": True, "workspace": {"id": 1}},
    {"class": "hidden", "pid": 99, "mapped": False, "workspace": {"id": 1}},
    {"class": "scratch", "pid": 50, "mapped": True, "workspace": {"id": -98}},
]


@patch("hsr.hypr.subprocess.run")
def test_clients_filters_unmapped_pidless_special(mock_run):
    mock_run.return_value = MagicMock(stdout=json.dumps(FAKE_CLIENTS), returncode=0)
    result = clients()
    assert [c["class"] for c in result] == ["foot"]
    mock_run.assert_called_once_with(
        ["hyprctl", "-j", "clients"], capture_output=True, text=True, check=True
    )


@patch("hsr.hypr.subprocess.run")
def test_dispatch_exec_builds_rule_prefix(mock_run):
    mock_run.return_value = MagicMock(returncode=0)
    dispatch_exec(["foot", "-e", "htop"], rules="workspace 2 silent")
    args = mock_run.call_args[0][0]
    assert args[:3] == ["hyprctl", "dispatch", "exec"]
    assert args[3].startswith("[workspace 2 silent]")
    assert "foot -e htop" in args[3]


@patch("hsr.hypr.subprocess.run")
def test_dispatch_exec_quotes_args_with_spaces(mock_run):
    mock_run.return_value = MagicMock(returncode=0)
    dispatch_exec(["sh", "-c", "cd /tmp && exec foot"], rules="")
    cmd = mock_run.call_args[0][0][3]
    assert "'cd /tmp && exec foot'" in cmd
```

- [ ] **Step 2: Run tests, verify fail**

Run: `python3 -m pytest tests/test_hypr.py -v`
Expected: FAIL — `ModuleNotFoundError: No module named 'hsr.hypr'`

- [ ] **Step 3: Implement**

`src/hsr/hypr.py`:
```python
import json
import shlex
import subprocess

SPECIAL_WORKSPACE_THRESHOLD = -90  # special workspaces have ids like -98/-99


def _hyprctl_json(cmd: str):
    res = subprocess.run(
        ["hyprctl", "-j", cmd], capture_output=True, text=True, check=True
    )
    return json.loads(res.stdout)


def clients() -> list[dict]:
    """Mapped, real-pid clients on regular workspaces."""
    return [
        c
        for c in _hyprctl_json("clients")
        if c.get("mapped")
        and c.get("pid", -1) > 0
        and c.get("workspace", {}).get("id", 0) > SPECIAL_WORKSPACE_THRESHOLD
    ]


def dispatch_exec(argv: list[str], rules: str = "") -> None:
    cmd = shlex.join(argv)
    if rules:
        cmd = f"[{rules}] {cmd}"
    subprocess.run(["hyprctl", "dispatch", "exec", cmd], capture_output=True, check=True)
```

- [ ] **Step 4: Run tests, verify pass**

Run: `python3 -m pytest tests/test_hypr.py -v` — Expected: 3 passed

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat: hyprctl wrapper - client listing and rule-prefixed exec dispatch"
```

---

### Task 5: `snapshot.py` — capture session to state file

**Files:**
- Create: `src/hsr/snapshot.py`
- Test: `tests/test_snapshot.py`

- [ ] **Step 1: Write the failing tests**

`tests/test_snapshot.py`:
```python
import json
import re
from unittest.mock import patch

from hsr.config import Config
from hsr.snapshot import take_snapshot, write_snapshot, default_state_path


FAKE_CLIENT = {
    "class": "foot",
    "title": "foot",
    "pid": 123,
    "workspace": {"id": 2, "name": "2"},
    "floating": True,
    "at": [10, 20],
    "size": [800, 600],
    "pinned": False,
    "fullscreen": 0,
    "mapped": True,
}


@patch("hsr.snapshot.proc.deepest_cwd", return_value="/home/x/proj")
@patch("hsr.snapshot.proc.cmdline", return_value=["foot"])
@patch("hsr.snapshot.hypr.clients", return_value=[FAKE_CLIENT])
def test_take_snapshot(mock_clients, mock_cmd, mock_cwd):
    snap = take_snapshot(Config())
    assert snap["version"] == 1
    [c] = snap["clients"]
    assert c == {
        "class": "foot",
        "title": "foot",
        "workspace": 2,
        "floating": True,
        "at": [10, 20],
        "size": [800, 600],
        "pinned": False,
        "fullscreen": 0,
        "cmdline": ["foot"],
        "cwd": "/home/x/proj",
    }


@patch("hsr.snapshot.proc.deepest_cwd", return_value=None)
@patch("hsr.snapshot.proc.cmdline", return_value=[])
@patch("hsr.snapshot.hypr.clients", return_value=[FAKE_CLIENT])
def test_take_snapshot_skips_clients_without_cmdline(mock_clients, mock_cmd, mock_cwd):
    assert take_snapshot(Config())["clients"] == []


@patch("hsr.snapshot.proc.deepest_cwd", return_value=None)
@patch("hsr.snapshot.proc.cmdline", return_value=["rofi"])
@patch("hsr.snapshot.hypr.clients", return_value=[dict(FAKE_CLIENT, **{"class": "Rofi"})])
def test_take_snapshot_respects_exclude(mock_clients, mock_cmd, mock_cwd):
    cfg = Config(exclude=[re.compile("^rofi$", re.IGNORECASE)])
    assert take_snapshot(cfg)["clients"] == []


def test_write_snapshot_atomic(tmp_path):
    target = tmp_path / "state" / "session.json"
    write_snapshot({"version": 1, "clients": []}, target)
    assert json.loads(target.read_text()) == {"version": 1, "clients": []}
    assert list(target.parent.iterdir()) == [target]  # no leftover tmp file


def test_default_state_path_respects_xdg(monkeypatch, tmp_path):
    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path))
    assert default_state_path() == tmp_path / "hyprland-session-restore" / "session.json"
```

- [ ] **Step 2: Run tests, verify fail**

Run: `python3 -m pytest tests/test_snapshot.py -v`
Expected: FAIL — `ModuleNotFoundError: No module named 'hsr.snapshot'`

- [ ] **Step 3: Implement**

`src/hsr/snapshot.py`:
```python
import json
import os
from pathlib import Path

from hsr import hypr, proc
from hsr.config import Config


def default_state_path() -> Path:
    base = Path(os.environ.get("XDG_STATE_HOME", Path.home() / ".local" / "state"))
    return base / "hyprland-session-restore" / "session.json"


def take_snapshot(cfg: Config) -> dict:
    out = []
    for c in hypr.clients():
        if cfg.is_excluded(c["class"]):
            continue
        argv = proc.cmdline(c["pid"])
        if not argv:
            continue
        out.append(
            {
                "class": c["class"],
                "title": c.get("title", ""),
                "workspace": c["workspace"]["id"],
                "floating": c.get("floating", False),
                "at": c.get("at", [0, 0]),
                "size": c.get("size", [0, 0]),
                "pinned": c.get("pinned", False),
                "fullscreen": c.get("fullscreen", 0),
                "cmdline": argv,
                "cwd": proc.deepest_cwd(c["pid"]),
            }
        )
    return {"version": 1, "clients": out}


def write_snapshot(snap: dict, path: Path) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_suffix(".tmp")
    tmp.write_text(json.dumps(snap, indent=2))
    tmp.replace(path)
```

- [ ] **Step 4: Run tests, verify pass**

Run: `python3 -m pytest tests/test_snapshot.py -v` — Expected: 5 passed

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat: session snapshot - capture clients to atomic JSON state file"
```

---

### Task 6: `restore.py` — relaunch and position clients

**Files:**
- Create: `src/hsr/restore.py`
- Test: `tests/test_restore.py`

Restore logic per client:
- Skip if class excluded by config.
- argv = launch_overrides.get(class) or saved cmdline.
- If cwd saved: wrap as `["sh", "-c", "cd <quoted cwd> && exec <quoted argv>"]`.
- Rules: `workspace <id> silent`; if floating add `;float;move X Y;size W H`; if pinned add `;pin`; if fullscreen truthy add `;fullscreen`.
- No dedup/already-running checks — restore assumes fresh login. `--dry-run` prints instead of dispatching.

- [ ] **Step 1: Write the failing tests**

`tests/test_restore.py`:
```python
import re
from unittest.mock import patch

from hsr.config import Config
from hsr.restore import restore_session, build_launch


CLIENT = {
    "class": "foot",
    "workspace": 3,
    "floating": False,
    "at": [10, 20],
    "size": [800, 600],
    "pinned": False,
    "fullscreen": 0,
    "cmdline": ["foot"],
    "cwd": "/home/x/proj",
}


def test_build_launch_wraps_cwd_in_sh():
    argv, rules = build_launch(CLIENT, Config())
    assert argv[0:2] == ["sh", "-c"]
    assert argv[2] == "cd /home/x/proj && exec foot"
    assert rules == "workspace 3 silent"


def test_build_launch_no_cwd():
    c = dict(CLIENT, cwd=None)
    argv, rules = build_launch(c, Config())
    assert argv == ["foot"]


def test_build_launch_floating_rules():
    c = dict(CLIENT, floating=True, pinned=True, fullscreen=1)
    _, rules = build_launch(c, Config())
    assert rules == "workspace 3 silent;float;move 10 20;size 800 600;pin;fullscreen"


def test_build_launch_override():
    cfg = Config(launch_overrides={"foot": ["kitty"]})
    argv, _ = build_launch(dict(CLIENT, cwd=None), cfg)
    assert argv == ["kitty"]


@patch("hsr.restore.hypr.dispatch_exec")
def test_restore_session_dispatches_and_skips_excluded(mock_exec):
    snap = {
        "version": 1,
        "clients": [CLIENT, dict(CLIENT, **{"class": "Rofi", "cmdline": ["rofi"]})],
    }
    cfg = Config(exclude=[re.compile("^rofi$", re.IGNORECASE)])
    n = restore_session(snap, cfg)
    assert n == 1
    assert mock_exec.call_count == 1


@patch("hsr.restore.hypr.dispatch_exec")
def test_restore_dry_run_does_not_dispatch(mock_exec, capsys):
    n = restore_session({"version": 1, "clients": [CLIENT]}, Config(), dry_run=True)
    assert n == 1
    mock_exec.assert_not_called()
    assert "foot" in capsys.readouterr().out
```

- [ ] **Step 2: Run tests, verify fail**

Run: `python3 -m pytest tests/test_restore.py -v`
Expected: FAIL — `ModuleNotFoundError: No module named 'hsr.restore'`

- [ ] **Step 3: Implement**

`src/hsr/restore.py`:
```python
import shlex

from hsr import hypr
from hsr.config import Config


def build_launch(client: dict, cfg: Config) -> tuple[list[str], str]:
    argv = cfg.launch_overrides.get(client["class"]) or client["cmdline"]
    if client.get("cwd"):
        argv = ["sh", "-c", f"cd {shlex.quote(client['cwd'])} && exec {shlex.join(argv)}"]
    rules = f"workspace {client['workspace']} silent"
    if client.get("floating"):
        x, y = client["at"]
        w, h = client["size"]
        rules += f";float;move {x} {y};size {w} {h}"
    if client.get("pinned"):
        rules += ";pin"
    if client.get("fullscreen"):
        rules += ";fullscreen"
    return argv, rules


def restore_session(snap: dict, cfg: Config, dry_run: bool = False) -> int:
    count = 0
    for client in snap.get("clients", []):
        if cfg.is_excluded(client["class"]):
            continue
        argv, rules = build_launch(client, cfg)
        if dry_run:
            print(f"[{rules}] {shlex.join(argv)}")
        else:
            hypr.dispatch_exec(argv, rules)
        count += 1
    return count
```

- [ ] **Step 4: Run tests, verify pass**

Run: `python3 -m pytest tests/test_restore.py -v` — Expected: 6 passed

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat: session restore - rule-prefixed relaunch with cwd wrapping"
```

---

### Task 7: `cli.py` — snapshot / restore / watch subcommands

**Files:**
- Create: `src/hsr/cli.py`
- Test: `tests/test_cli.py`

- [ ] **Step 1: Write the failing tests**

`tests/test_cli.py`:
```python
import json
from unittest.mock import patch

from hsr.cli import main


@patch("hsr.cli.take_snapshot", return_value={"version": 1, "clients": []})
def test_snapshot_writes_state(mock_take, tmp_path):
    state = tmp_path / "s.json"
    rc = main(["snapshot", "--state-file", str(state)])
    assert rc == 0
    assert json.loads(state.read_text()) == {"version": 1, "clients": []}


@patch("hsr.cli.restore_session", return_value=2)
def test_restore_reads_state(mock_restore, tmp_path, capsys):
    state = tmp_path / "s.json"
    state.write_text('{"version": 1, "clients": []}')
    rc = main(["restore", "--state-file", str(state)])
    assert rc == 0
    assert "2" in capsys.readouterr().out


def test_restore_missing_state_errors(tmp_path, capsys):
    rc = main(["restore", "--state-file", str(tmp_path / "missing.json")])
    assert rc == 1
    assert "no snapshot" in capsys.readouterr().err.lower()


@patch("hsr.cli.take_snapshot", return_value={"version": 1, "clients": []})
@patch("hsr.cli.time.sleep", side_effect=KeyboardInterrupt)
def test_watch_loops_until_interrupt(mock_sleep, mock_take, tmp_path):
    rc = main(["watch", "--state-file", str(tmp_path / "s.json"), "--interval", "5"])
    assert rc == 0
    mock_take.assert_called_once()
    mock_sleep.assert_called_once_with(5)
```

- [ ] **Step 2: Run tests, verify fail**

Run: `python3 -m pytest tests/test_cli.py -v`
Expected: FAIL — `ModuleNotFoundError: No module named 'hsr.cli'`

- [ ] **Step 3: Implement**

`src/hsr/cli.py`:
```python
import argparse
import json
import sys
import time
from pathlib import Path

from hsr import __version__
from hsr.config import load_config
from hsr.restore import restore_session
from hsr.snapshot import default_state_path, take_snapshot, write_snapshot


def _parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        prog="hsr", description="Snapshot and restore Hyprland sessions"
    )
    p.add_argument("--version", action="version", version=__version__)
    p.add_argument("--config", type=Path, help="path to config.toml")
    sub = p.add_subparsers(dest="command", required=True)

    for name, desc in [
        ("snapshot", "capture current session"),
        ("restore", "relaunch saved session"),
        ("watch", "snapshot periodically"),
    ]:
        sp = sub.add_parser(name, help=desc)
        sp.add_argument("--state-file", type=Path, default=default_state_path())
    sub.choices["restore"].add_argument("--dry-run", action="store_true")
    sub.choices["watch"].add_argument(
        "--interval", type=int, help="seconds between snapshots (default: config)"
    )
    return p


def main(argv: list[str] | None = None) -> int:
    args = _parser().parse_args(argv)
    cfg = load_config(args.config)

    if args.command == "snapshot":
        write_snapshot(take_snapshot(cfg), args.state_file)
        return 0

    if args.command == "restore":
        if not args.state_file.is_file():
            print(f"no snapshot at {args.state_file}", file=sys.stderr)
            return 1
        snap = json.loads(args.state_file.read_text())
        n = restore_session(snap, cfg, dry_run=args.dry_run)
        print(f"restored {n} clients")
        return 0

    if args.command == "watch":
        interval = args.interval or cfg.interval
        try:
            while True:
                write_snapshot(take_snapshot(cfg), args.state_file)
                time.sleep(interval)
        except KeyboardInterrupt:
            return 0

    return 2


if __name__ == "__main__":
    sys.exit(main())
```

- [ ] **Step 4: Run tests, verify pass**

Run: `python3 -m pytest tests/test_cli.py -v` — Expected: 4 passed
Also full suite: `python3 -m pytest` — all green.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat: CLI - snapshot, restore (--dry-run), watch subcommands"
```

---

### Task 8: README, example config, systemd units

**Files:**
- Modify: `README.md` (full rewrite)
- Create: `examples/config.toml`
- Create: `systemd/hyprland-session-restore.service`
- Create: `systemd/hyprland-session-restore.timer`

- [ ] **Step 1: Write files**

`examples/config.toml`: the config example from the plan header (exclude, launch_overrides, snapshot.interval), with comments.

`systemd/hyprland-session-restore.service`:
```ini
[Unit]
Description=Hyprland session snapshot
ConditionEnvironment=HYPRLAND_INSTANCE_SIGNATURE

[Service]
Type=oneshot
ExecStart=%h/.local/bin/hsr snapshot
```

`systemd/hyprland-session-restore.timer`:
```ini
[Unit]
Description=Periodic Hyprland session snapshot

[Timer]
OnUnitActiveSec=60
OnActiveSec=60

[Install]
WantedBy=graphical-session.target
```

`README.md` — keep the vibecoded badge at top, then sections:
- Pitch (X11 session management died with Wayland; this restores it for Hyprland).
- How it works: `hyprctl -j clients` + `/proc/PID` cmdline + deepest-descendant cwd; restore via `hyprctl dispatch exec "[workspace N silent;...]"` wrapping in `sh -c 'cd ... && exec ...'` so terminals reopen where you were working.
- Install: `pipx install .` or `pip install --user .`
- Usage: `hsr snapshot`, `hsr restore [--dry-run]`, `hsr watch --interval 60`.
- Autostart via `hyprland.conf`:
  ```
  exec-once = hsr restore
  exec-once = hsr watch
  ```
  or systemd user timer (copy units to `~/.config/systemd/user/`, `systemctl --user enable --now hyprland-session-restore.timer`).
- Configuration: full config.toml reference (exclude regexes, launch_overrides, interval).
- Limitations: Hyprland-only; apps must be restartable from cmdline; no window *content* restore; exact tiling layout not reproduced (workspace + floating geometry only).
- License MIT.

- [ ] **Step 2: Verify full test suite green**

Run: `python3 -m pytest` — Expected: all passed.

- [ ] **Step 3: Commit**

```bash
git add -A && git commit -m "docs: full README, example config, systemd units"
```
