#!/usr/bin/env python3
"""Detect LOZA components whose manifest version increased."""

from __future__ import annotations

import argparse
import json
import re
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
SEMVER_RE = re.compile(r"^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:[-+][0-9A-Za-z.-]+)?$")


def run_git(args: list[str], check: bool = True) -> str:
    completed = subprocess.run(["git", *args], cwd=ROOT, text=True, capture_output=True)
    if check and completed.returncode != 0:
        raise RuntimeError(completed.stderr.strip() or completed.stdout.strip())
    return completed.stdout


def load_yaml_text(text: str, source: str) -> dict[str, Any]:
    data = yaml.safe_load(text)
    if not isinstance(data, dict):
        raise ValueError(f"{source}: expected YAML mapping")
    return data


def load_current_yaml(path: str) -> dict[str, Any]:
    return load_yaml_text((ROOT / path).read_text(encoding="utf-8"), path)


def load_base_yaml(ref: str, path: str) -> dict[str, Any] | None:
    completed = subprocess.run(["git", "show", f"{ref}:{path}"], cwd=ROOT, text=True, capture_output=True)
    if completed.returncode != 0:
        return None
    return load_yaml_text(completed.stdout, f"{ref}:{path}")


def parse_semver(version: Any, component: str) -> tuple[int, int, int, str]:
    value = str(version)
    match = SEMVER_RE.match(value)
    if value.startswith("v") or not match:
        raise ValueError(f"{component}: invalid semver {value!r}; use X.Y.Z without leading v")
    return int(match.group(1)), int(match.group(2)), int(match.group(3)), value


def load_registry() -> dict[str, str]:
    registry = load_current_yaml("release.yaml")
    components = registry.get("components")
    if not isinstance(components, dict) or not components:
        raise ValueError("release.yaml must contain a non-empty components mapping")
    return {str(component): str(path) for component, path in components.items()}


def detect(base_ref: str, components_filter: set[str] | None = None) -> dict[str, dict[str, Any]]:
    registry = load_registry()
    result: dict[str, dict[str, Any]] = {}
    errors: list[str] = []

    for component, manifest_path in registry.items():
        if components_filter and component not in components_filter:
            continue
        try:
            current = load_current_yaml(manifest_path)
            base = load_base_yaml(base_ref, manifest_path)
            _, _, _, new_version = parse_semver(current.get("version"), component)
            old_version = None
            changed = base is None
            if base is not None:
                _, _, _, old_version = parse_semver(base.get("version"), component)
                changed = old_version != new_version
                if changed and parse_semver(current.get("version"), component)[:3] <= parse_semver(base.get("version"), component)[:3]:
                    errors.append(f"{component}: version decreased or did not increase ({old_version} -> {new_version})")

            result[component] = {
                "changed": changed,
                "old_version": old_version,
                "new_version": new_version,
                "manifest": manifest_path,
                "kind": current.get("kind"),
                "language": current.get("language"),
            }
        except Exception as exc:
            errors.append(str(exc))

    if errors:
        for error in errors:
            print(f"ERROR: {error}", file=sys.stderr)
        raise SystemExit(1)

    return result


def compact_matrix(changes: dict[str, dict[str, Any]], include_unchanged: bool = False) -> dict[str, list[dict[str, Any]]]:
    include = []
    for component, data in changes.items():
        if include_unchanged or data["changed"]:
            include.append({"component": component, **data})
    return {"include": include}


def write_github_output(values: dict[str, str]) -> None:
    output_path = Path(str(__import__("os").environ.get("GITHUB_OUTPUT", "")))
    if not output_path:
        return
    with output_path.open("a", encoding="utf-8") as handle:
        for key, value in values.items():
            handle.write(f"{key}={value}\n")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-ref", required=True, help="Git ref/SHA to compare against")
    parser.add_argument("--component", action="append", help="Limit detection to selected component(s)")
    parser.add_argument("--matrix-only", action="store_true", help="Print only the compact changed-component matrix")
    parser.add_argument("--github-output", action="store_true", help="Write common outputs to $GITHUB_OUTPUT")
    args = parser.parse_args()

    changes = detect(args.base_ref, set(args.component or []) or None)
    matrix = compact_matrix(changes)
    changed = [name for name, data in changes.items() if data["changed"]]
    spec_changed = "spec" in changed

    if args.github_output:
        write_github_output(
            {
                "changes": json.dumps(changes, separators=(",", ":")),
                "matrix": json.dumps(matrix, separators=(",", ":")),
                "changed_components": ",".join(changed),
                "has_changes": str(bool(changed)).lower(),
                "spec_changed": str(spec_changed).lower(),
            }
        )

    print(json.dumps(matrix if args.matrix_only else changes, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
