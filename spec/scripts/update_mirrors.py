#!/usr/bin/env python3
"""Synchronize compatibility mirror files from canonical spec sources."""

from __future__ import annotations

from pathlib import Path

from check_mirrors import MIRROR_PAIRS


def main() -> int:
    spec_root = Path(__file__).resolve().parents[1]

    for canonical_rel, mirror_rel in MIRROR_PAIRS:
        canonical = spec_root / canonical_rel
        mirror = spec_root / mirror_rel

        if not canonical.exists():
            raise FileNotFoundError(f"missing canonical file: {canonical_rel}")

        mirror.parent.mkdir(parents=True, exist_ok=True)
        mirror.write_text(canonical.read_text(encoding="utf-8"), encoding="utf-8")
        print(f"synced {mirror_rel} <- {canonical_rel}")

    print("\n✓ mirrors updated")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
