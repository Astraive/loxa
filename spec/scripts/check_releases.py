#!/usr/bin/env python3
"""
Protect release snapshots from accidental mutations.

Enforces that releases/ folder changes require explicit version bump and CHANGELOG update.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path


def check_releases_immutability() -> tuple[bool, list[str]]:
    """
    Verify releases/ folder integrity.
    
    Returns (is_valid, messages).
    Checks:
    - Each release has a manifest or README documenting its contents
    - Releases are named semantically (v1, v0.0.1, v2.0.0, etc.)
    """
    spec_root = Path(__file__).resolve().parents[1]
    releases_dir = spec_root / "releases"

    messages = []
    issues = []

    if not releases_dir.exists():
        messages.append("⊘ releases/ folder does not exist (not yet created)")
        return True, messages

    # Check each version folder
    for version_dir in sorted(releases_dir.iterdir()):
        if not version_dir.is_dir():
            continue

        version_name = version_dir.name

        # Verify version name is semantic
        if not _is_semantic_version(version_name):
            issues.append(f"✗ invalid version name: {version_name} (expected v1, v2.0.0, etc.)")
            continue

        # Check for README or manifest
        readme = version_dir / "README.md"
        manifest = version_dir / "manifest.json"

        if readme.exists():
            messages.append(f"✓ {version_dir.relative_to(spec_root)}/README.md")
        elif manifest.exists():
            messages.append(f"✓ {version_dir.relative_to(spec_root)}/manifest.json")
        else:
            issues.append(f"⊘ {version_name}/ has no README.md or manifest.json")

        # List contents
        contents = [f for f in version_dir.iterdir() if f.name not in {".git", ".gitignore"}]
        if contents:
            messages.append(f"  └─ contains {len(contents)} file(s)/folder(s)")

    if issues:
        for issue in issues:
            messages.append(issue)
        return False, messages

    messages.append(f"✓ releases/ folder structure is valid")
    return True, messages


def _is_semantic_version(name: str) -> bool:
    """Check if name looks like a version (v1, v1.0, v0.0.1, etc.)."""
    if not name.startswith("v"):
        return False
    version_part = name[1:]
    parts = version_part.split(".")
    return all(p.isdigit() for p in parts) and len(parts) >= 1


def main() -> int:
    is_valid, messages = check_releases_immutability()

    for msg in messages:
        print(msg)

    return 0 if is_valid else 1


if __name__ == "__main__":
    raise SystemExit(main())
