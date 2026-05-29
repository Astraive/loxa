#!/usr/bin/env python3
"""Create component-specific LOXA release tags safely."""

from __future__ import annotations

import argparse
import subprocess
import sys
from pathlib import Path
from typing import Any

try:
    import yaml
except ImportError:  # pragma: no cover
    print("PyYAML is required: python -m pip install pyyaml", file=sys.stderr)
    sys.exit(2)


ROOT = Path(__file__).resolve().parents[2]
DEFAULT_PREFIXES = {
    "loxa": "v",
    "spec": "spec/v",
    "collector": "collector/v",
    "cortex": "cortex/v",
    "cli": "cli/v",
    "lql": "lql/v",
    "sdk-go": "sdks/go/v",
    "sdk-js": "sdks/js/v",
    "sdk-py": "sdks/py/v",
    "sdk-rs": "sdks/rs/v",
}


def run_git(args: list[str], check: bool = True) -> subprocess.CompletedProcess[str]:
    completed = subprocess.run(["git", *args], cwd=ROOT, text=True, capture_output=True)
    if check and completed.returncode != 0:
        raise RuntimeError(completed.stderr.strip() or completed.stdout.strip())
    return completed


def load_yaml(path: Path) -> dict[str, Any]:
    data = yaml.safe_load(path.read_text(encoding="utf-8"))
    if not isinstance(data, dict):
        raise ValueError(f"{path}: expected YAML mapping")
    return data


def component_manifest(component: str) -> Path:
    registry = load_yaml(ROOT / "release.yaml")
    components = registry.get("components")
    if not isinstance(components, dict) or component not in components:
        raise ValueError(f"unknown component {component!r}")
    return ROOT / str(components[component])


def tag_prefix(component: str, manifest: dict[str, Any]) -> str:
    publish = manifest.get("publish") if isinstance(manifest.get("publish"), dict) else {}
    for key in ("go", "github_release"):
        section = publish.get(key)
        if isinstance(section, dict) and section.get("tag_prefix"):
            return str(section["tag_prefix"])
    return DEFAULT_PREFIXES[component]


def tag_exists(tag: str) -> bool:
    return run_git(["rev-parse", "-q", "--verify", f"refs/tags/{tag}"], check=False).returncode == 0


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("component", choices=sorted(DEFAULT_PREFIXES))
    parser.add_argument("--dry-run", action="store_true")
    parser.add_argument("--push", action="store_true", help="Push the tag to origin after creating it")
    parser.add_argument("--message", help="Annotated tag message")
    args = parser.parse_args()

    try:
        manifest_path = component_manifest(args.component)
        manifest = load_yaml(manifest_path)
        version = str(manifest["version"])
        if version.startswith("v"):
            raise ValueError("manifest version must not start with v")
        tag = f"{tag_prefix(args.component, manifest)}{version}"

        if tag_exists(tag):
            raise ValueError(f"tag already exists: {tag}")

        print(tag)
        if args.dry_run:
            return 0

        message = args.message or f"Release {args.component} {version}"
        run_git(["tag", "-a", tag, "-m", message])
        if args.push:
            run_git(["push", "origin", tag])
        return 0
    except Exception as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
