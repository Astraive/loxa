#!/usr/bin/env python3
"""Check that compatibility mirror files are in sync with canonical sources."""

from __future__ import annotations

from pathlib import Path
from typing import Iterable


MIRROR_PAIRS: tuple[tuple[str, str], ...] = (
    ("spec/schemas/json/event.schema.json", "schema/event.schema.json"),
    ("spec/schemas/json/event.strict.schema.json", "schema/event.strict.schema.json"),
    ("spec/schemas/json/event.loose.schema.json", "schema/event.loose.schema.json"),
    ("spec/schemas/json/ingest-envelope.schema.json", "schema/ingest.schema.json"),
    ("spec/schemas/json/collector-response.schema.json", "schema/collector-response.schema.json"),
    ("spec/openapi/collector.openapi.yaml", "openapi/collector.openapi.yaml"),
    ("spec/proto/loxa/v1/collector.proto", "proto/loxa/v1/collector.proto"),
    ("spec/proto/loxa/v1/event.proto", "proto/loxa/v1/event.proto"),
    ("spec/proto/loxa/v1/ingest.proto", "proto/loxa/v1/ingest.proto"),
    ("spec/proto/loxa/v1/cortex.proto", "proto/loxa/v1/cortex.proto"),
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
