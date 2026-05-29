#!/usr/bin/env python3
"""Create component-specific LOXA release tags safely."""

from __future__ import annotations

import argparse
import os
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

DEFAULT_GIT_AUTHOR_NAME = "github-actions[bot]"
DEFAULT_GIT_AUTHOR_EMAIL = "41898282+github-actions[bot]@users.noreply.github.com"


def run_git(args: list[str], check: bool = True) -> subprocess.CompletedProcess[str]:
    completed = subprocess.run(
        ["git", *args],
        cwd=ROOT,
        text=True,
        capture_output=True,
    )

    if check and completed.returncode != 0:
        message = completed.stderr.strip() or completed.stdout.strip()
        raise RuntimeError(message)

    return completed


def load_yaml(path: Path) -> dict[str, Any]:
    if not path.exists():
        raise FileNotFoundError(f"missing YAML file: {path}")

    data = yaml.safe_load(path.read_text(encoding="utf-8"))

    if not isinstance(data, dict):
        raise ValueError(f"{path}: expected YAML mapping")

    return data


def component_manifest(component: str) -> Path:
    registry_path = ROOT / "release.yaml"
    registry = load_yaml(registry_path)

    components = registry.get("components")

    if not isinstance(components, dict):
        raise ValueError(f"{registry_path}: expected 'components' mapping")

    if component not in components:
        raise ValueError(f"unknown component {component!r}")

    return ROOT / str(components[component])


def tag_prefix(component: str, manifest: dict[str, Any]) -> str:
    publish = manifest.get("publish")

    if not isinstance(publish, dict):
        publish = {}

    for key in ("go", "github_release"):
        section = publish.get(key)

        if isinstance(section, dict) and section.get("tag_prefix"):
            return str(section["tag_prefix"])

    if component not in DEFAULT_PREFIXES:
        raise ValueError(f"missing default tag prefix for component {component!r}")

    return DEFAULT_PREFIXES[component]


def local_tag_exists(tag: str) -> bool:
    return (
        run_git(
            ["rev-parse", "-q", "--verify", f"refs/tags/{tag}"],
            check=False,
        ).returncode
        == 0
    )


def remote_tag_exists(tag: str) -> bool:
    completed = run_git(
        ["ls-remote", "--exit-code", "--tags", "origin", f"refs/tags/{tag}"],
        check=False,
    )

    return completed.returncode == 0


def tag_exists(tag: str) -> bool:
    return local_tag_exists(tag) or remote_tag_exists(tag)


def git_config_get(key: str) -> str:
    completed = run_git(["config", "--get", key], check=False)

    if completed.returncode != 0:
        return ""

    return completed.stdout.strip()


def configure_git_author() -> None:
    """Ensure git can create annotated tags in CI runners."""

    existing_name = git_config_get("user.name")
    existing_email = git_config_get("user.email")

    name = (
        existing_name
        or os.environ.get("GIT_AUTHOR_NAME")
        or os.environ.get("GITHUB_ACTOR")
        or DEFAULT_GIT_AUTHOR_NAME
    )

    email = (
        existing_email
        or os.environ.get("GIT_AUTHOR_EMAIL")
        or DEFAULT_GIT_AUTHOR_EMAIL
    )

    run_git(["config", "user.name", name])
    run_git(["config", "user.email", email])


def validate_version(version: str) -> None:
    if not version:
        raise ValueError("manifest version is empty")

    if version.startswith("v"):
        raise ValueError("manifest version must not start with v")


def create_tag(tag: str, message: str) -> None:
    configure_git_author()
    run_git(["tag", "-a", tag, "-m", message])


def push_tag(tag: str) -> None:
    run_git(["push", "origin", tag])


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("component", choices=sorted(DEFAULT_PREFIXES))
    parser.add_argument("--dry-run", action="store_true")
    parser.add_argument(
        "--push",
        action="store_true",
        help="Push the tag to origin after creating it",
    )
    parser.add_argument("--message", help="Annotated tag message")

    args = parser.parse_args()

    try:
        manifest_path = component_manifest(args.component)
        manifest = load_yaml(manifest_path)

        version = str(manifest.get("version", "")).strip()
        validate_version(version)

        tag = f"{tag_prefix(args.component, manifest)}{version}"

        if tag_exists(tag):
            raise ValueError(f"tag already exists: {tag}")

        print(tag)

        if args.dry_run:
            return 0

        message = args.message or f"Release {args.component} {version}"

        create_tag(tag, message)

        if args.push:
            push_tag(tag)

        return 0

    except Exception as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())