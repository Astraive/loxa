"""Run the deterministic SDK coverage gate from this package directory."""

from __future__ import annotations

import subprocess
import sys


def main() -> int:
    commands = (
        [sys.executable, "-m", "coverage", "erase"],
        [
            sys.executable,
            "-m",
            "coverage",
            "run",
            "--branch",
            "--source=src/loza",
            "-m",
            "pytest",
            "--ignore=tests/test_e2e_live.py",
        ],
        [
            sys.executable,
            "-m",
            "coverage",
            "html",
        ],
        [
            sys.executable,
            "-m",
            "coverage",
            "json",
        ],
        [
            sys.executable,
            "-m",
            "coverage",
            "report",
            "--fail-under=95",
            "--show-missing",
        ],
    )
    for command in commands:
        completed = subprocess.run(command, check=False)
        if completed.returncode:
            return completed.returncode
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
