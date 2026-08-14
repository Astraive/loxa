#!/usr/bin/env python3
"""Validate LOZA release manifests."""

from __future__ import annotations

import argparse
import json
import re
import sys
import tomllib
from pathlib import Path
from typing import Any

try:
    import yaml
except ImportError:  # pragma: no cover
    print("PyYAML is required: python -m pip install pyyaml", file=sys.stderr)
    sys.exit(2)


ROOT = Path(__file__).resolve().parents[2]
SEMVER_RE = re.compile(r"^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:[-+][0-9A-Za-z.-]+)?$")
REQUIRED_FIELDS = ("name", "kind", "version", "description", "license", "repository", "module")
KNOWN_KINDS = {"spec", "docker", "cli", "sdk", "package", "release"}
KNOWN_LANGUAGES = {"go", "javascript", "python", "rust"}
PUBLISH_OWNER = "astraive"
NPM_PACKAGE = "@astraive/loza"


def load_yaml(path: Path) -> dict[str, Any]:
    with path.open("r", encoding="utf-8") as handle:
        data = yaml.safe_load(handle)
    if not isinstance(data, dict):
        raise ValueError(f"{path}: expected a YAML mapping")
    return data


def load_registry() -> dict[str, str]:
    registry_path = ROOT / "release.yaml"
    if not registry_path.exists():
        raise ValueError("release.yaml is missing")
    registry = load_yaml(registry_path)
    components = registry.get("components")
    if not isinstance(components, dict) or not components:
        raise ValueError("release.yaml must contain a non-empty 'components' mapping")
    return {str(name): str(path) for name, path in components.items()}


def require_mapping(value: Any, field: str, component: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise ValueError(f"{component}: '{field}' must be a mapping")
    return value


def validate_native_metadata(component: str, manifest: dict[str, Any], manifest_path: Path) -> list[str]:
    errors: list[str] = []
    version = str(manifest["version"])
    paths = manifest.get("paths") if isinstance(manifest.get("paths"), dict) else {}
    root = ROOT / str(paths.get("root", manifest_path.parent.as_posix()))

    if component == "sdk-js":
        package_json = root / "package.json"
        if package_json.exists():
            package_data = json.loads(package_json.read_text(encoding="utf-8"))
            if package_data.get("name") != NPM_PACKAGE:
                errors.append(f"{component}: sdks/js/package.json name must be {NPM_PACKAGE!r}, not {package_data.get('name')!r}")
            if package_data.get("version") != version:
                errors.append(f"{component}: sdks/js/package.json version must match manifest version {version}")

    if component == "sdk-py":
        pyproject = root / "pyproject.toml"
        if pyproject.exists():
            project = tomllib.loads(pyproject.read_text(encoding="utf-8")).get("project", {})
            if project.get("name") != "loza":
                errors.append(f"{component}: sdks/py/pyproject.toml project.name must be 'loza'")
            if project.get("version") != version:
                errors.append(f"{component}: sdks/py/pyproject.toml version must match manifest version {version}")

    if component == "sdk-rs":
        cargo_toml = root / "Cargo.toml"
        if cargo_toml.exists():
            package = tomllib.loads(cargo_toml.read_text(encoding="utf-8")).get("package", {})
            if package.get("name") != "loza":
                errors.append(f"{component}: sdks/rs/Cargo.toml package.name must be 'loza'")
            if package.get("version") != version:
                errors.append(f"{component}: sdks/rs/Cargo.toml version must match manifest version {version}")

    return errors


def validate_publish_metadata(component: str, manifest: dict[str, Any], manifest_path: Path) -> list[str]:
    errors: list[str] = []
    kind = manifest["kind"]
    language = manifest.get("language")
    publish = manifest.get("publish")
    if not isinstance(publish, dict):
        return [f"{component}: publish metadata is required"]

    for channel, metadata in publish.items():
        if isinstance(metadata, dict) and "owner" in metadata and metadata["owner"] != PUBLISH_OWNER:
            errors.append(f"{component}: publish.{channel}.owner must be '{PUBLISH_OWNER}'")

    if kind == "docker":
        docker = publish.get("docker")
        if not isinstance(docker, dict):
            errors.append(f"{component}: publish.docker is required for docker components")
        else:
            if docker.get("owner") != PUBLISH_OWNER:
                errors.append(f"{component}: publish.docker.owner must be '{PUBLISH_OWNER}'")
            images = docker.get("images")
            if not isinstance(images, list) or not all(isinstance(item, str) and item for item in images):
                errors.append(f"{component}: publish.docker.images must be a non-empty string list")
            dockerfile = manifest.get("paths", {}).get("dockerfile") or docker.get("dockerfile")
            if not dockerfile:
                errors.append(f"{component}: paths.dockerfile or publish.docker.dockerfile is required")
            elif not (ROOT / str(dockerfile)).exists():
                errors.append(f"{component}: dockerfile does not exist: {dockerfile}")

    if kind == "cli":
        release = publish.get("github_release")
        go = publish.get("go")
        if not isinstance(release, dict) or release.get("binary") != "loza":
            errors.append(f"{component}: publish.github_release.binary must be 'loza'")
        elif release.get("owner") != PUBLISH_OWNER:
            errors.append(f"{component}: publish.github_release.owner must be '{PUBLISH_OWNER}'")
        if not isinstance(go, dict) or not go.get("install"):
            errors.append(f"{component}: publish.go.install is required")
        elif go.get("owner") != PUBLISH_OWNER:
            errors.append(f"{component}: publish.go.owner must be '{PUBLISH_OWNER}'")

    if kind == "spec":
        go = publish.get("go")
        if not isinstance(go, dict) or go.get("tag_prefix") != "spec/v":
            errors.append(f"{component}: publish.go.tag_prefix must be 'spec/v'")
        elif go.get("owner") != PUBLISH_OWNER:
            errors.append(f"{component}: publish.go.owner must be '{PUBLISH_OWNER}'")

    if kind == "release":
        release = publish.get("github_release")
        if not isinstance(release, dict):
            errors.append(f"{component}: release components require publish.github_release")
        elif release.get("tag_prefix") != "v":
            errors.append(f"{component}: umbrella release tag prefix must be 'v'")
        elif release.get("owner") != PUBLISH_OWNER:
            errors.append(f"{component}: publish.github_release.owner must be '{PUBLISH_OWNER}'")

    if kind == "sdk":
        if language not in KNOWN_LANGUAGES:
            errors.append(f"{component}: sdk language must be one of {sorted(KNOWN_LANGUAGES)}")
        if language == "go" and not isinstance(publish.get("go"), dict):
            errors.append(f"{component}: Go SDK requires publish.go")
        if language == "go" and isinstance(publish.get("go"), dict) and publish["go"].get("owner") != PUBLISH_OWNER:
            errors.append(f"{component}: publish.go.owner must be '{PUBLISH_OWNER}'")
        if language == "javascript":
            npm = publish.get("npm")
            if not isinstance(npm, dict):
                errors.append(f"{component}: JavaScript SDK requires publish.npm")
            elif npm.get("package") != NPM_PACKAGE:
                errors.append(f"{component}: npm package must be {NPM_PACKAGE!r}, not {npm.get('package')!r}")
            elif npm.get("owner") != PUBLISH_OWNER:
                errors.append(f"{component}: publish.npm.owner must be '{PUBLISH_OWNER}'")
        if language == "python":
            pypi = publish.get("pypi")
            if not isinstance(pypi, dict) or pypi.get("package") != "loza":
                errors.append(f"{component}: PyPI package must be 'loza'")
            elif pypi.get("owner") != PUBLISH_OWNER:
                errors.append(f"{component}: publish.pypi.owner must be '{PUBLISH_OWNER}'")
        if language == "rust":
            crates = publish.get("crates")
            if not isinstance(crates, dict) or crates.get("package") != "loza":
                errors.append(f"{component}: crates.io package must be 'loza'")
            elif crates.get("owner") != PUBLISH_OWNER:
                errors.append(f"{component}: publish.crates.owner must be '{PUBLISH_OWNER}'")

    if kind == "package" and language == "rust":
        if not isinstance(publish.get("github_release"), dict):
            errors.append(f"{component}: package components require publish.github_release")
        elif publish["github_release"].get("owner") != PUBLISH_OWNER:
            errors.append(f"{component}: publish.github_release.owner must be '{PUBLISH_OWNER}'")

    errors.extend(validate_native_metadata(component, manifest, manifest_path))
    return errors


def validate_component(component: str, manifest_rel: str) -> list[str]:
    errors: list[str] = []
    manifest_path = ROOT / manifest_rel
    if not manifest_path.exists():
        return [f"{component}: manifest does not exist: {manifest_rel}"]
    try:
        manifest = load_yaml(manifest_path)
    except Exception as exc:
        return [f"{component}: failed to load {manifest_rel}: {exc}"]

    for field in REQUIRED_FIELDS:
        if field not in manifest or manifest[field] in (None, ""):
            errors.append(f"{component}: required field '{field}' is missing")

    if errors:
        return errors

    version = str(manifest["version"])
    if version.startswith("v") or not SEMVER_RE.match(version):
        errors.append(f"{component}: version must be semantic version without leading v, got {version!r}")
    if manifest["kind"] not in KNOWN_KINDS:
        errors.append(f"{component}: kind must be one of {sorted(KNOWN_KINDS)}, got {manifest['kind']!r}")
    if not isinstance(manifest.get("paths", {}), dict):
        errors.append(f"{component}: paths must be a mapping when present")

    errors.extend(validate_publish_metadata(component, manifest, manifest_path))
    return errors


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--component", action="append", help="Validate only this component. May be repeated.")
    args = parser.parse_args()

    try:
        components = load_registry()
    except Exception as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1

    selected = set(args.component or components.keys())
    errors: list[str] = []
    for component, manifest in components.items():
        if component in selected:
            errors.extend(validate_component(component, manifest))

    unknown = selected - set(components)
    errors.extend(f"{component}: unknown component in release.yaml" for component in sorted(unknown))

    if errors:
        print("Manifest validation failed:", file=sys.stderr)
        for error in errors:
            print(f"  - {error}", file=sys.stderr)
        return 1

    print(f"Validated {len(selected)} release manifest(s).")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
