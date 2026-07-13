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
