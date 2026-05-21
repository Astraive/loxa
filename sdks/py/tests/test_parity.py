from __future__ import annotations

import json
from pathlib import Path

import loxa


REQUIRED_DIRS = [
    ".github/workflows",
    "bench",
    "cmd",
    "core",
    "examples/basic",
    "examples/custom_schema",
    "examples/fastapi",
    "examples/flask",
    "integrations",
    "internal/core",
    "internal/env",
    "internal/jsonenc",
    "internal/pool",
    "internal/safe",
    "libs",
    "src/loxa/middleware/asgi",
    "src/loxa/middleware/django",
    "src/loxa/middleware/fastapi",
    "src/loxa/middleware/flask",
    "src/loxa/middleware/starlette",
    "packages",
    "src/loxa/sinks/httpbatch",
    "storagepath",
    "testkit",
    "tests/conformance",
    "tests/integration",
    "tests/root",
    "utils",
]


def test_required_repo_tree_exists() -> None:
    root = Path(__file__).resolve().parents[1]
    missing = [path for path in REQUIRED_DIRS if not (root / path).is_dir()]
    assert missing == []


def test_public_api_matches_superset_manifest() -> None:
    root = Path(__file__).resolve().parents[2]
    manifest = json.loads((root / "loxa-spec" / "docs" / "sdk-parity-manifest.json").read_text())
    required = set()
    for key, values in manifest.items():
        if isinstance(values, list) and key not in {"excluded_from_sdk", "sdks"}:
            required.update(values)
    missing = sorted(name for name in required if not hasattr(loxa, name))
    assert missing == []


def test_spec_manifest_is_present() -> None:
    root = Path(__file__).resolve().parents[2]
    manifest_path = root / "loxa-spec" / "examples" / "golden" / "manifest.json"
    manifest = json.loads(manifest_path.read_text())
    assert (manifest_path.parent / manifest["strict_schema"]).exists()
    assert (manifest_path.parent / manifest["loose_schema"]).exists()
