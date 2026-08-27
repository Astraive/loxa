"""Load SDK version from loza-py.yaml metadata file.

Falls back to a hardcoded default if the file cannot be found or parsed.
"""

from __future__ import annotations

_FALLBACK_VERSION = "0.4.0"


def _load_version() -> str:
    """Read version from loza-py.yaml, searching standard locations."""
    from pathlib import Path

    candidates = [
        Path(__file__).resolve().parent.parent.parent / "loza-py.yaml",  # relative to src/loza/version.py
        Path.cwd() / "loza-py.yaml",
    ]
    for path in candidates:
        if path.is_file():
            try:
                text = path.read_text(encoding="utf-8")
                for line in text.splitlines():
                    stripped = line.strip()
                    if stripped.startswith("version:"):
                        value = stripped.split(":", 1)[1].strip().strip("\"'")
                        if value:
                            return value
            except OSError:
                continue
    return _FALLBACK_VERSION


SDK_VERSION: str = _load_version()
