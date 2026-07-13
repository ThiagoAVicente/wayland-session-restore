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
