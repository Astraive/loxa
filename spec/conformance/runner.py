#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import subprocess
import sys
import time
import urllib.request
from dataclasses import dataclass
from pathlib import Path

LOXA_SPEC_ROOT = Path(__file__).resolve().parents[1]
WORKSPACE_ROOT = LOXA_SPEC_ROOT.parent

# TS loader import hook — works on Node 18+ (replaces --experimental-strip-types which requires Node 22.6+)
_TS_IMPORT_FLAG = (
    'data:text/javascript,import { register } from "node:module";'
    ' import { pathToFileURL } from "node:url";'
    ' register("./scripts/ts-loader.mjs", pathToFileURL("./"));'
)


@dataclass(frozen=True)
class Check:
    sdk: str
    group: str
    command: tuple[str, ...]
    cwd: Path
    description: str


@dataclass(frozen=True)
class CheckResult:
    sdk: str
    group: str
    description: str
    command: tuple[str, ...]
    cwd: str
    passed: bool
    returncode: int
    duration_s: float
    stdout: str
    stderr: str


SDK_GROUPS: dict[str, list[Check]] = {
    "javascript": [
        Check("javascript", "state_machine", ("node", "--import", _TS_IMPORT_FLAG, "--test", "--test-concurrency=1", "tests/event.test.ts"), WORKSPACE_ROOT / "sdks" / "js", "core lifecycle state machine"),
        Check("javascript", "canonical_fields", ("node", "--import", _TS_IMPORT_FLAG, "--test", "--test-concurrency=1", "tests/conformance.test.ts"), WORKSPACE_ROOT / "sdks" / "js", "canonical field ownership"),
        Check("javascript", "duplicate_policy", ("node", "--import", _TS_IMPORT_FLAG, "--test", "--test-concurrency=1", "tests/conformance.test.ts"), WORKSPACE_ROOT / "sdks" / "js", "duplicate policy and behavior"),
        Check("javascript", "sampling", ("node", "--import", _TS_IMPORT_FLAG, "--test", "--test-concurrency=1", "tests/sampler.test.ts"), WORKSPACE_ROOT / "sdks" / "js", "sampling behavior"),
        Check("javascript", "delivery_semantics", ("node", "--import", _TS_IMPORT_FLAG, "--test", "--test-concurrency=1", "tests/sink.test.ts"), WORKSPACE_ROOT / "sdks" / "js", "delivery and sink semantics"),
        Check("javascript", "panic_error_safety", ("node", "--import", _TS_IMPORT_FLAG, "--test", "--test-concurrency=1", "tests/event.test.ts"), WORKSPACE_ROOT / "sdks" / "js", "panic-safe error paths"),
        Check("javascript", "config_precedence", ("node", "--import", _TS_IMPORT_FLAG, "--test", "--test-concurrency=1", "tests/facade.test.ts"), WORKSPACE_ROOT / "sdks" / "js", "config and facade behavior"),
        Check("javascript", "metrics", ("node", "--import", _TS_IMPORT_FLAG, "--test", "--test-concurrency=1", "tests/sink.test.ts"), WORKSPACE_ROOT / "sdks" / "js", "metrics-adjacent checks"),
        Check("javascript", "golden_fixtures", ("node", "--import", _TS_IMPORT_FLAG, "--test", "--test-concurrency=1", "tests/conformance.test.ts"), WORKSPACE_ROOT / "sdks" / "js", "shared fixture conformance"),
        Check("javascript", "collector_integration", ("node", "--import", _TS_IMPORT_FLAG, "--test", "--test-concurrency=1", "tests/e2e-collector.test.ts"), WORKSPACE_ROOT / "sdks" / "js", "collector integration"),
        Check("javascript", "cortex_emitted_shape", ("node", "--import", _TS_IMPORT_FLAG, "--test", "--test-concurrency=1", "tests/emitted-shape.test.ts"), WORKSPACE_ROOT / "sdks" / "js", "Cortex-consumable emitted event shape"),
        Check("javascript", "parity", ("node", "--import", _TS_IMPORT_FLAG, "--test", "--test-concurrency=1", "tests/conformance.test.ts"), WORKSPACE_ROOT / "sdks" / "js", "API parity"),
    ],
    "go": [
        Check("go", "state_machine", ("go", "test", "./src/core"), WORKSPACE_ROOT / "sdks" / "go", "core lifecycle and state transitions"),
        Check("go", "canonical_fields", ("go", "test", "./src/core"), WORKSPACE_ROOT / "sdks" / "go", "canonical field ownership and collisions"),
        Check("go", "duplicate_policy", ("go", "test", "./tests/conformance", "-run", "TestPublicAPISurfaceCoreFlow|TestConformanceFixtures"), WORKSPACE_ROOT / "sdks" / "go", "duplicate policy and public lifecycle flow"),
        Check("go", "sampling", ("go", "test", "./src/core"), WORKSPACE_ROOT / "sdks" / "go", "sampler and emission behavior"),
        Check("go", "delivery_semantics", ("go", "test", "./src/core", "-run", "TestCollectorAckBehaviorFixtures|TestSharedEmittedShapeFixture|Test.*HTTPTransport.*"), WORKSPACE_ROOT / "sdks" / "go", "collector ack and transport retry semantics"),
        Check("go", "panic_error_safety", ("go", "test", "./src/core"), WORKSPACE_ROOT / "sdks" / "go", "panic-safe error paths"),
        Check("go", "config_precedence", ("go", "test", "./src/core", "-run", "Test.*Config.*"), WORKSPACE_ROOT / "sdks" / "go", "config and env precedence"),
        Check("go", "metrics", ("go", "test", "./src/core", "-run", "Test.*Stats.*|Test.*Metrics.*"), WORKSPACE_ROOT / "sdks" / "go", "stats and metrics hooks"),
        Check("go", "golden_fixtures", ("go", "test", "./tests/conformance", "-run", "TestConformanceFixtures"), WORKSPACE_ROOT / "sdks" / "go", "shared fixture and manifest checks"),
        Check("go", "collector_integration", ("go", "test", "."), WORKSPACE_ROOT / "sdks" / "go" / "tests" / "integration", "end-to-end collector delivery"),
        Check("go", "cortex_emitted_shape", ("go", "test", "./src/core", "-run", "TestSharedEmittedShapeFixture"), WORKSPACE_ROOT / "sdks" / "go", "Cortex-consumable emitted event shape"),
        Check("go", "parity", ("go", "test", "./tests/conformance", "-run", "TestPublicAPISurfaceCoreFlow|TestRootModuleDependencyBoundary"), WORKSPACE_ROOT / "sdks" / "go", "stable-v1 manifest and dependency boundary"),
    ],
    "python": [
        Check("python", "state_machine", (sys.executable, "-m", "pytest", "-q", "tests/test_state_machine.py"), WORKSPACE_ROOT / "sdks" / "py", "core lifecycle state machine"),
        Check("python", "canonical_fields", (sys.executable, "-m", "pytest", "-q", "tests/test_canonical_fields.py"), WORKSPACE_ROOT / "sdks" / "py", "canonical field ownership"),
        Check("python", "duplicate_policy", (sys.executable, "-m", "pytest", "-q", "tests/test_behavior.py", "tests/test_canonical_fields.py"), WORKSPACE_ROOT / "sdks" / "py", "duplicate field handling and general behavior"),
        Check("python", "sampling", (sys.executable, "-m", "pytest", "-q", "tests/test_behavior.py"), WORKSPACE_ROOT / "sdks" / "py", "sampling and lifecycle behavior"),
        Check("python", "delivery_semantics", (sys.executable, "-m", "pytest", "-q", "tests/test_delivery_semantics.py", "tests/test_httpbatch_sink.py", "tests/test_collector_ack_fixture.py", "tests/test_ingest_envelope_fixture.py"), WORKSPACE_ROOT / "sdks" / "py", "collector delivery and ack handling"),
        Check("python", "panic_error_safety", (sys.executable, "-m", "pytest", "-q", "tests/test_panic_safety.py"), WORKSPACE_ROOT / "sdks" / "py", "panic-safe error behavior"),
        Check("python", "config_precedence", (sys.executable, "-m", "pytest", "-q", "tests/test_config_loader.py"), WORKSPACE_ROOT / "sdks" / "py", "config and env precedence"),
        Check("python", "metrics", (sys.executable, "-m", "pytest", "-q", "tests/test_metrics_export.py"), WORKSPACE_ROOT / "sdks" / "py", "stats and metrics exports"),
        Check("python", "golden_fixtures", (sys.executable, "-m", "pytest", "-q", "tests/test_spec_conformance.py", "tests/test_emitted_shape_fixture.py"), WORKSPACE_ROOT / "sdks" / "py", "shared fixture conformance"),
        Check("python", "collector_integration", (sys.executable, "-m", "pytest", "-q", "tests/test_integration.py", "tests/test_middleware_integration.py"), WORKSPACE_ROOT / "sdks" / "py", "collector and middleware integration"),
        Check("python", "cortex_emitted_shape", (sys.executable, "-m", "pytest", "-q", "tests/test_emitted_shape_fixture.py"), WORKSPACE_ROOT / "sdks" / "py", "Cortex-consumable emitted event shape"),
        Check("python", "parity", (sys.executable, "-m", "pytest", "-q", "tests/test_parity.py"), WORKSPACE_ROOT / "sdks" / "py", "stable-v1 API parity vs superset manifest"),
    ],
    "rust": [
        Check("rust", "state_machine", ("cargo", "test", "-q", "--test", "state_machine"), WORKSPACE_ROOT / "sdks" / "rs", "core lifecycle state machine"),
        Check("rust", "canonical_fields", ("cargo", "test", "-q", "--test", "api_behavior_test"), WORKSPACE_ROOT / "sdks" / "rs", "canonical fields and context behavior"),
        Check("rust", "duplicate_policy", ("cargo", "test", "-q", "--test", "behavior"), WORKSPACE_ROOT / "sdks" / "rs", "duplicate policy and behavior"),
        Check("rust", "sampling", ("cargo", "test", "-q", "--test", "behavior"), WORKSPACE_ROOT / "sdks" / "rs", "sampling and lifecycle behavior"),
        Check("rust", "delivery_semantics", ("cargo", "test", "-q", "--test", "collector_ack_fixture", "--test", "httpbatch_transport", "--test", "ingest_envelope_fixture"), WORKSPACE_ROOT / "sdks" / "rs", "collector delivery and ack handling"),
        Check("rust", "panic_error_safety", ("cargo", "test", "-q", "--test", "smoke"), WORKSPACE_ROOT / "sdks" / "rs", "panic-safe error behavior"),
        Check("rust", "config_precedence", ("cargo", "test", "-q", "--test", "config_defaults"), WORKSPACE_ROOT / "sdks" / "rs", "config defaults and precedence"),
        Check("rust", "metrics", ("cargo", "test", "-q", "--test", "e2e"), WORKSPACE_ROOT / "sdks" / "rs", "runtime behavior and metric-adjacent checks"),
        Check("rust", "golden_fixtures", ("cargo", "test", "-q", "--test", "spec_contract_test", "--test", "conformance_fixtures"), WORKSPACE_ROOT / "sdks" / "rs", "shared fixture and manifest checks"),
        Check("rust", "collector_integration", ("cargo", "test", "-q", "--test", "integration", "--test", "e2e"), WORKSPACE_ROOT / "sdks" / "rs", "collector delivery and integration"),
        Check("rust", "cortex_emitted_shape", ("cargo", "test", "-q", "--test", "collector_cortex_conformance", "--test", "emitted_shape_fixture"), WORKSPACE_ROOT / "sdks" / "rs", "Cortex-consumable emitted event shape"),
        Check("rust", "parity", ("cargo", "test", "-q", "--test", "parity"), WORKSPACE_ROOT / "sdks" / "rs", "stable-v1 API parity vs superset manifest"),
    ],
}


def _all_groups() -> list[str]:
    groups = {check.group for checks in SDK_GROUPS.values() for check in checks}
    return sorted(groups)


def _run_check(check: Check, verbose: bool) -> CheckResult:
    print(f"[{check.sdk}:{check.group}] {' '.join(check.command)}")
    print(f"  {check.description}")
    started = time.perf_counter()
    result = subprocess.run(
        list(check.command),
        cwd=check.cwd,
        capture_output=not verbose,
        text=True,
    )
    duration_s = time.perf_counter() - started
    if result.returncode == 0:
        print("  PASS")
        return CheckResult(
            sdk=check.sdk,
            group=check.group,
            description=check.description,
            command=check.command,
            cwd=str(check.cwd),
            passed=True,
            returncode=0,
            duration_s=duration_s,
            stdout=result.stdout or "",
            stderr=result.stderr or "",
        )
    print("  FAIL")
    if not verbose:
        output = (result.stdout or "") + (result.stderr or "")
        if output.strip():
            print(output.strip()[:5000])
    return CheckResult(
        sdk=check.sdk,
        group=check.group,
        description=check.description,
        command=check.command,
        cwd=str(check.cwd),
        passed=False,
        returncode=result.returncode,
        duration_s=duration_s,
        stdout=result.stdout or "",
        stderr=result.stderr or "",
    )


def _is_collector_reachable() -> bool:
    """Check if the collector is reachable at its health endpoint."""
    try:
        req = urllib.request.Request("http://127.0.0.1:9308/healthz", method="GET")
        with urllib.request.urlopen(req, timeout=3):
            return True
    except Exception:
        return False


def _is_python_sdk_installed() -> bool:
    """Check if the loxa Python SDK can be imported."""
    result = subprocess.run(
        [sys.executable, "-c", "import loxa"],
        capture_output=True,
        text=True,
    )
    return result.returncode == 0


def _skip_result(check: Check, reason: str) -> CheckResult:
    """Create a CheckResult representing a skipped check."""
    print(f"[{check.sdk}:{check.group}] SKIP ({reason})")
    return CheckResult(
        sdk=check.sdk,
        group=check.group,
        description=check.description,
        command=check.command,
        cwd=str(check.cwd),
        passed=True,
        returncode=0,
        duration_s=0.0,
        stdout="",
        stderr="",
    )


def _selected_checks(sdk: str, group: str) -> list[Check]:
    sdks = list(SDK_GROUPS.keys()) if sdk == "all" else [sdk]
    groups = _all_groups() if group == "all" else [group]
    selected: list[Check] = []
    for sdk_name in sdks:
        for check in SDK_GROUPS[sdk_name]:
            if check.group in groups:
                selected.append(check)
    return selected


def _print_matrix() -> None:
    groups = _all_groups()
    header = "group".ljust(24) + "".join(sdk.rjust(12) for sdk in ("go", "python", "rust", "javascript"))
    print(header)
    print("-" * len(header))
    for group in groups:
        row = group.ljust(24)
        for sdk in ("go", "python", "rust", "javascript"):
            present = any(check.group == group for check in SDK_GROUPS[sdk])
            row += ("yes" if present else "-").rjust(10)
        print(row)


def _print_json(results: list[CheckResult]) -> None:
    payload = {
        "checks": [
            {
                "sdk": result.sdk,
                "group": result.group,
                "description": result.description,
                "command": list(result.command),
                "cwd": result.cwd,
                "passed": result.passed,
                "returncode": result.returncode,
                "duration_s": result.duration_s,
                "stdout": result.stdout,
                "stderr": result.stderr,
            }
            for result in results
        ]
    }
    payload["summary"] = {
        "total": len(results),
        "passed": sum(1 for result in results if result.passed),
        "failed": sum(1 for result in results if not result.passed),
    }
    print(json.dumps(payload, indent=2))


def main() -> int:
    parser = argparse.ArgumentParser(description="Run grouped LOXA SDK conformance checks.")
    parser.add_argument("--sdk", choices=("all", "go", "python", "rust", "javascript"), default="all")
    parser.add_argument("--group", choices=("all", *_all_groups()), default="all")
    parser.add_argument("--verbose", action="store_true")
    parser.add_argument("--matrix", action="store_true", help="Print grouped conformance matrix and exit.")
    parser.add_argument("--json", action="store_true", help="Print structured per-check results as JSON.")
    args = parser.parse_args()

    if args.matrix:
        _print_matrix()
        return 0

    checks = _selected_checks(args.sdk, args.group)
    results: list[CheckResult] = []

    # Pre-check: is the collector reachable? (cached, only checked if needed)
    collector_ok: bool | None = None
    python_sdk_ok: bool | None = None

    for check in checks:
        if check.group == "collector_integration":
            if collector_ok is None:
                collector_ok = _is_collector_reachable()
            if not collector_ok:
                results.append(_skip_result(check, "no collector at 127.0.0.1:9308"))
                continue

        if check.sdk == "python":
            if python_sdk_ok is None:
                python_sdk_ok = _is_python_sdk_installed()
            if not python_sdk_ok:
                results.append(_skip_result(check, "python sdk not installed"))
                continue

        results.append(_run_check(check, args.verbose))
    if args.json:
        _print_json(results)
    passed = all(result.passed for result in results)
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
