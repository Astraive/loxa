#!/usr/bin/env python3
"""Validate conformance fixtures against canonical JSON Schemas."""

from __future__ import annotations

import json
import sys
import warnings
from pathlib import Path

if sys.stdout.encoding and sys.stdout.encoding.lower() != "utf-8":
    sys.stdout.reconfigure(encoding="utf-8")  # type: ignore[union-attr]
    sys.stderr.reconfigure(encoding="utf-8")  # type: ignore[union-attr]

warnings.filterwarnings("ignore", category=DeprecationWarning, module="jsonschema")
warnings.filterwarnings(
    "ignore",
    category=DeprecationWarning,
    message=r".*RefResolver is deprecated.*",
)

from jsonschema import Draft202012Validator, RefResolver


_SCHEMA_DIR = Path(__file__).resolve().parents[1] / "schema"


def _load_json(path: Path) -> object:
    with path.open("r", encoding="utf-8") as handle:
        return json.load(handle)


def _build_schema_store() -> dict[str, object]:
    """Build a URI -> schema dict mapping every schema's $id to its content.

    This lets RefResolver resolve $ref URLs locally instead of fetching
    them over HTTP (the $id values are GitHub URLs that return 404).
    """
    store: dict[str, object] = {}
    for schema_file in sorted(_SCHEMA_DIR.glob("*.json")):
        schema = _load_json(schema_file)
        if isinstance(schema, dict) and "$id" in schema:
            store[str(schema["$id"])] = schema
    return store


def _build_validator(
    schema_path: Path,
    schema_override: dict[str, object] | None = None,
    store: dict[str, object] | None = None,
) -> Draft202012Validator:
    schema = _load_json(schema_path)
    if schema_override is not None:
        schema = schema_override
    resolver = RefResolver(
        base_uri=schema_path.resolve().as_uri(),
        referrer=schema,
        store=store or {},
    )
    return Draft202012Validator(
        schema,
        resolver=resolver,
        format_checker=Draft202012Validator.FORMAT_CHECKER,
    )


def _must_exist(manifest_dir: Path, rel_paths: list[str], seen: set[str], label: str) -> list[Path]:
    resolved: list[Path] = []
    for rel in rel_paths:
        path = (manifest_dir / rel).resolve()
        if not path.exists():
            raise FileNotFoundError(f"{label}: fixture missing: {path}")
        seen.add(path.as_posix())
        resolved.append(path)
    return resolved


def _validate_pass(paths: list[Path], validator: Draft202012Validator, label: str) -> None:
    for path in paths:
        payload = _load_json(path)
        errors = sorted(validator.iter_errors(payload), key=lambda err: err.path)
        if errors:
            first = errors[0]
            raise AssertionError(
                f"{label}: expected valid fixture {path} but got: {first.message}"
            )
        print(f"✓ {label} pass {path}")


def _validate_fail(paths: list[Path], validator: Draft202012Validator, label: str) -> None:
    for path in paths:
        payload = _load_json(path)
        errors = sorted(validator.iter_errors(payload), key=lambda err: err.path)
        if not errors:
            raise AssertionError(f"{label}: expected invalid fixture {path} to fail validation")
        print(f"✓ {label} fail {path}")


def _assert_fixture_coverage(spec_root: Path, referenced: set[str]) -> None:
    required_dirs = (
        spec_root / "fixtures" / "valid",
        spec_root / "fixtures" / "invalid",
        spec_root / "fixtures" / "ingest",
        spec_root / "fixtures" / "collector-responses",
        spec_root / "fixtures" / "emitted-shape",
    )
    for directory in required_dirs:
        if not directory.exists():
            continue
        for fixture in sorted(directory.glob("*.json")):
            key = fixture.resolve().as_posix()
            if key not in referenced:
                raise AssertionError(f"fixture not referenced by conformance manifest: {fixture}")


def _validate_size_limit_fail(paths: list[Path], max_event_bytes: int, label: str) -> None:
    for path in paths:
        size = path.stat().st_size
        payload = _load_json(path)
        message = payload.get("message") if isinstance(payload, dict) else None
        message_len = len(message) if isinstance(message, str) else 0
        if size <= max_event_bytes and message_len < 800:
            raise AssertionError(
                f"{label}: expected fixture {path} to exceed limits (size={size}, message_len={message_len})"
            )
        print(f"✓ {label} fail {path}")


def _validate_emitted_shape(paths: list[Path]) -> None:
    for path in paths:
        payload = _load_json(path)
        if not isinstance(payload, dict):
            raise AssertionError(f"emitted_shape: fixture must be an object: {path}")
        expected = payload.get("expected")
        if not isinstance(expected, dict):
            raise AssertionError(f"emitted_shape: missing expected block: {path}")
        present = expected.get("present")
        equals = expected.get("equals")
        if not isinstance(present, list) or not present:
            raise AssertionError(f"emitted_shape: expected.present must be non-empty list: {path}")
        if not isinstance(equals, dict) or not equals:
            raise AssertionError(f"emitted_shape: expected.equals must be non-empty object: {path}")
        print(f"✓ emitted_shape pass {path}")


def _validate_collector_behavior(paths: list[Path], validator: Draft202012Validator) -> None:
    for path in paths:
        payload = _load_json(path)
        if not isinstance(payload, dict):
            raise AssertionError(f"collector_response: fixture must be an object: {path}")
        schema_target = payload
        if "response" in payload:
            response = payload.get("response")
            expected = payload.get("expected")
            if not isinstance(response, dict):
                raise AssertionError(f"collector_response: response must be object: {path}")
            if not isinstance(expected, dict):
                raise AssertionError(f"collector_response: expected must be object: {path}")
            schema_target = response
        errors = sorted(validator.iter_errors(schema_target), key=lambda err: err.path)
        if errors:
            first = errors[0]
            raise AssertionError(
                f"collector_response: expected valid fixture {path} but got: {first.message}"
            )
        print(f"✓ collector_response pass {path}")


def _validate_ingest_fixtures(
    paths: list[Path],
    ingest_validator: Draft202012Validator,
    event_validator: Draft202012Validator,
) -> None:
    for path in paths:
        payload = _load_json(path)
        if not isinstance(payload, dict):
            raise AssertionError(f"ingest: fixture must be an object: {path}")
        mode = payload.get("mode")
        if mode == "wrapped_batch":
            body = payload.get("body")
            if not isinstance(body, dict):
                raise AssertionError(f"ingest: wrapped_batch fixture missing object body: {path}")
            errors = sorted(ingest_validator.iter_errors(body), key=lambda err: err.path)
            if errors:
                raise AssertionError(f"ingest: wrapped_batch invalid {path}: {errors[0].message}")
            print(f"✓ ingest pass {path}")
            continue
        if mode == "single_event":
            body = payload.get("body")
            if not isinstance(body, dict):
                raise AssertionError(f"ingest: single_event fixture missing object body: {path}")
            errors = sorted(event_validator.iter_errors(body), key=lambda err: err.path)
            if errors:
                raise AssertionError(f"ingest: single_event invalid {path}: {errors[0].message}")
            print(f"✓ ingest pass {path}")
            continue
        if mode == "ndjson":
            lines = payload.get("body_lines")
            if not isinstance(lines, list) or not lines:
                raise AssertionError(f"ingest: ndjson fixture missing body_lines: {path}")
            for index, line in enumerate(lines):
                try:
                    event_payload = json.loads(str(line))
                except json.JSONDecodeError as exc:
                    raise AssertionError(f"ingest: ndjson line {index} in {path} is invalid JSON") from exc
                errors = sorted(event_validator.iter_errors(event_payload), key=lambda err: err.path)
                if errors:
                    raise AssertionError(
                        f"ingest: ndjson line {index} in {path} failed event schema: {errors[0].message}"
                    )
            print(f"✓ ingest pass {path}")
            continue
        errors = sorted(ingest_validator.iter_errors(payload), key=lambda err: err.path)
        if errors:
            raise AssertionError(f"ingest: expected valid fixture {path} but got: {errors[0].message}")
        print(f"✓ ingest pass {path}")


def _validate_ingest_invalid(paths: list[Path], validator: Draft202012Validator) -> None:
    for path in paths:
        payload = _load_json(path)
        errors = sorted(validator.iter_errors(payload), key=lambda err: err.path)
        if not errors:
            raise AssertionError(f"ingest_invalid: expected invalid fixture {path} to fail validation")
        print(f"✓ ingest_invalid fail {path}")


def main() -> int:
    spec_root = Path(__file__).resolve().parents[1]
    manifest_path = spec_root / "conformance" / "manifest.json"
    if not manifest_path.exists():
        manifest_path = spec_root / "examples" / "golden" / "manifest.json"
    manifest = _load_json(manifest_path)
    if not isinstance(manifest, dict):
        raise TypeError("manifest must be an object")
    manifest_dir = manifest_path.parent

    strict_schema = (manifest_dir / str(manifest["strict_schema"])).resolve()
    loose_schema = (manifest_dir / str(manifest["loose_schema"])).resolve()
    event_schema = (spec_root / "schema" / "event.schema.json").resolve()
    ingest_schema = (spec_root / "schema" / "ingest.schema.json").resolve()
    collector_schema = (spec_root / "schema" / "collector-response.schema.json").resolve()

    if not strict_schema.exists() or not loose_schema.exists():
        raise FileNotFoundError("manifest schema paths must exist")

    event_schema_payload = _load_json(event_schema)
    if not isinstance(event_schema_payload, dict):
        raise TypeError("event schema must be an object")
    strict_event_schema = dict(event_schema_payload)
    strict_event_schema["additionalProperties"] = False

    schema_store = _build_schema_store()

    loose_validator = _build_validator(event_schema, store=schema_store)
    strict_validator = _build_validator(event_schema, schema_override=strict_event_schema, store=schema_store)
    ingest_validator = _build_validator(ingest_schema, store=schema_store)
    collector_validator = _build_validator(collector_schema, store=schema_store)

    seen: set[str] = set()
    valid = _must_exist(manifest_dir, list(manifest.get("valid", [])), seen, "valid")
    loose_only_valid = _must_exist(
        manifest_dir, list(manifest.get("loose_only_valid", [])), seen, "loose_only_valid"
    )
    invalid = _must_exist(manifest_dir, list(manifest.get("invalid", [])), seen, "invalid")
    strict_only_invalid = _must_exist(
        manifest_dir, list(manifest.get("strict_only_invalid", [])), seen, "strict_only_invalid"
    )
    invalid_ingest = _must_exist(
        manifest_dir, list(manifest.get("invalid_ingest", [])), seen, "invalid_ingest"
    )
    invalid_collector_response = _must_exist(
        manifest_dir,
        list(manifest.get("invalid_collector_response", [])),
        seen,
        "invalid_collector_response",
    )
    emitted_shape = _must_exist(
        manifest_dir, list(manifest.get("emitted_shape", [])), seen, "emitted_shape"
    )
    invalid_limits = _must_exist(
        manifest_dir, list(manifest.get("invalid_limits", [])), seen, "invalid_limits"
    )
    collector_ack_behavior = _must_exist(
        manifest_dir, list(manifest.get("collector_ack_behavior", [])), seen, "collector_ack_behavior"
    )
    ingest_envelopes = _must_exist(
        manifest_dir, list(manifest.get("ingest_envelopes", [])), seen, "ingest_envelopes"
    )

    _assert_fixture_coverage(spec_root, seen)

    _validate_pass(valid, loose_validator, "event_loose")
    _validate_pass(valid, strict_validator, "event_strict")
    _validate_pass(loose_only_valid, loose_validator, "event_loose_only")
    _validate_fail(loose_only_valid, strict_validator, "event_strict_only")
    _validate_fail(invalid, loose_validator, "event_loose_invalid")
    _validate_fail(invalid, strict_validator, "event_strict_invalid")
    _validate_pass(strict_only_invalid, loose_validator, "event_loose_strict_only")
    _validate_fail(strict_only_invalid, strict_validator, "event_strict_invalid_only")
    _validate_emitted_shape(emitted_shape)
    _validate_collector_behavior(collector_ack_behavior, collector_validator)
    _validate_fail(invalid_collector_response, collector_validator, "collector_response_invalid")
    _validate_ingest_fixtures(ingest_envelopes, ingest_validator, loose_validator)
    _validate_ingest_invalid(invalid_ingest, ingest_validator)
    contract_path = (spec_root / "generated" / "contract" / "loza-contract.json").resolve()
    max_event_bytes = 65536
    if contract_path.exists():
        contract_payload = _load_json(contract_path)
        if isinstance(contract_payload, dict):
            max_event_bytes = int(
                contract_payload.get("limits", {}).get("max_event_size_bytes", max_event_bytes)
            )
    _validate_size_limit_fail(invalid_limits, max_event_bytes, "event_limits_invalid")

    total = (
        len(valid)
        + len(loose_only_valid)
        + len(invalid)
        + len(strict_only_invalid)
        + len(invalid_ingest)
        + len(invalid_collector_response)
        + len(invalid_limits)
        + len(emitted_shape)
        + len(collector_ack_behavior)
        + len(ingest_envelopes)
    )
    print(f"\n✓ conformance fixtures verified: {total} checks")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
