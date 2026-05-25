#!/usr/bin/env python3
"""Check that compatibility mirror files are in sync with canonical sources."""

from __future__ import annotations

import sys
from pathlib import Path
from typing import Iterable

if sys.stdout.encoding and sys.stdout.encoding.lower() != "utf-8":
    sys.stdout.reconfigure(encoding="utf-8")  # type: ignore[union-attr]
    sys.stderr.reconfigure(encoding="utf-8")  # type: ignore[union-attr]


MIRROR_PAIRS: tuple[tuple[str, str], ...] = (
    # No active mirror pairs — releases/v1/schemas/ are frozen snapshots, not mirrors.
)


def _check_pair(spec_root: Path, canonical_rel: str, mirror_rel: str) -> bool:
    canonical = spec_root / canonical_rel
    mirror = spec_root / mirror_rel

    if not canonical.exists():
        print(f"✗ missing canonical file: {canonical_rel}")
        return False
    if not mirror.exists():
        print(f"✗ missing mirror file: {mirror_rel}")
        return False
    if canonical.read_text(encoding="utf-8") != mirror.read_text(encoding="utf-8"):
        print(f"✗ drift detected: {mirror_rel} != {canonical_rel}")
        return False

    print(f"✓ {mirror_rel} ↔ {canonical_rel}")
    return True


def check_mirrors(spec_root: Path, pairs: Iterable[tuple[str, str]]) -> bool:
    ok = True
    for canonical_rel, mirror_rel in pairs:
        if not _check_pair(spec_root, canonical_rel, mirror_rel):
            ok = False
    return ok


def main() -> int:
    spec_root = Path(__file__).resolve().parents[1]
    all_ok = check_mirrors(spec_root, MIRROR_PAIRS)

    if all_ok:
        print("\n✓ all mirrors are in sync")
        return 0
    print("\n✗ mirror drift detected. Run: python scripts/update_mirrors.py")
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
