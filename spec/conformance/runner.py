#!/usr/bin/env python3
from __future__ import annotations

import argparse
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path

LOXA_SPEC_ROOT = Path(__file__).resolve().parents[1]
WORKSPACE_ROOT = LOXA_SPEC_ROOT.parent


@dataclass(frozen=True)
class Check:
    sdk: str
    group: str
    command: tuple[str, ...]
    cwd: Path
    description: str


SDK_GROUPS: dict[str, list[Check]] = {
    "javascript": [
        Check("javascript", "state_machine", ("node", "--test", "--experimental-strip-types", "tests/event.test.ts"), WORKSPACE_ROOT / "loxa-js", "core lifecycle state machine"),
        Check("javascript", "canonical_fields", ("node", "--test", "--experimental-strip-types", "tests/conformance.test.ts"), WORKSPACE_ROOT / "loxa-js", "canonical field ownership"),
        Check("javascript", "duplicate_policy", ("node", "--test", "--experimental-strip-types", "tests/conformance.test.ts"), WORKSPACE_ROOT / "loxa-js", "duplicate policy and behavior"),
        Check("javascript", "sampling", ("node", "--test", "--experimental-strip-types", "tests/sampler.test.ts"), WORKSPACE_ROOT / "loxa-js", "sampling behavior"),
        Check("javascript", "delivery_semantics", ("node", "--test", "--experimental-strip-types", "tests/sink.test.ts"), WORKSPACE_ROOT / "loxa-js", "delivery and sink semantics"),
        Check("javascript", "panic_error_safety", ("node", "--test", "--experimental-strip-types", "tests/event.test.ts"), WORKSPACE_ROOT / "loxa-js", "panic-safe error paths"),
        Check("javascript", "config_precedence", ("node", "--test", "--experimental-strip-types", "tests/facade.test.ts"), WORKSPACE_ROOT / "loxa-js", "config and facade behavior"),
        Check("javascript", "metrics", ("node", "--test", "--experimental-strip-types", "tests/sink.test.ts"), WORKSPACE_ROOT / "loxa-js", "metrics-adjacent checks"),
        Check("javascript", "golden_fixtures", ("node", "--test", "--experimental-strip-types", "tests/conformance.test.ts"), WORKSPACE_ROOT / "loxa-js", "shared fixture conformance"),
        Check("javascript", "collector_integration", ("node", "--test", "--experimental-strip-types", "tests/e2e-collector.test.ts"), WORKSPACE_ROOT / "loxa-js", "collector integration"),
        Check("javascript", "cortex_emitted_shape", ("node", "--test", "--experimental-strip-types", "tests/emitted-shape.test.ts"), WORKSPACE_ROOT / "loxa-js", "Cortex-consumable emitted event shape"),
        Check("javascript", "parity", ("node", "--test", "--experimental-strip-types", "tests/conformance.test.ts"), WORKSPACE_ROOT / "loxa-js", "API parity"),
    ],
    "go": [
        Check("go", "state_machine", ("go", "test", "./src/core"), WORKSPACE_ROOT / "loxa-go", "core lifecycle and state transitions"),
        Check("go", "canonical_fields", ("go", "test", "./src/core"), WORKSPACE_ROOT / "loxa-go", "canonical field ownership and collisions"),
        Check("go", "duplicate_policy", ("go", "test", "./tests/conformance", "-run", "TestPublicAPISurfaceCoreFlow|TestConformanceFixtures"), WORKSPACE_ROOT / "loxa-go", "duplicate policy and public lifecycle flow"),
        Check("go", "sampling", ("go", "test", "./src/core"), WORKSPACE_ROOT / "loxa-go", "sampler and emission behavior"),
        Check("go", "delivery_semantics", ("go", "test", "./src/core", "-run", "TestCollectorAckBehaviorFixtures|TestSharedEmittedShapeFixture|Test.*HTTPTransport.*"), WORKSPACE_ROOT / "loxa-go", "collector ack and transport retry semantics"),
        Check("go", "panic_error_safety", ("go", "test", "./src/core"), WORKSPACE_ROOT / "loxa-go", "panic-safe error paths"),
        Check("go", "config_precedence", ("go", "test", "./src/core", "-run", "Test.*Config.*"), WORKSPACE_ROOT / "loxa-go", "config and env precedence"),
        Check("go", "metrics", ("go", "test", "./src/core", "-run", "Test.*Stats.*|Test.*Metrics.*"), WORKSPACE_ROOT / "loxa-go", "stats and metrics hooks"),
        Check("go", "golden_fixtures", ("go", "test", "./tests/conformance", "-run", "TestConformanceFixtures"), WORKSPACE_ROOT / "loxa-go", "shared fixture and manifest checks"),
        Check("go", "collector_integration", ("go", "test", "."), WORKSPACE_ROOT / "loxa-go/tests/integration", "end-to-end collector delivery"),
        Check("go", "cortex_emitted_shape", ("go", "test", "./src/core", "-run", "TestSharedEmittedShapeFixture"), WORKSPACE_ROOT / "loxa-go", "Cortex-consumable emitted event shape"),
        Check("go", "parity", ("go", "test", "./tests/conformance", "-run", "TestPublicAPISurfaceCoreFlow|TestRootModuleDependencyBoundary"), WORKSPACE_ROOT / "loxa-go", "stable-v1 manifest and dependency boundary"),
    ],
    "python": [
        Check("python", "state_machine", (sys.executable, "-m", "pytest", "-q", "tests/test_state_machine.py"), WORKSPACE_ROOT / "loxa-py", "core lifecycle state machine"),
        Check("python", "canonical_fields", (sys.executable, "-m", "pytest", "-q", "tests/test_canonical_fields.py"), WORKSPACE_ROOT / "loxa-py", "canonical field ownership"),
        Check("python", "duplicate_policy", (sys.executable, "-m", "pytest", "-q", "tests/test_behavior.py", "tests/test_canonical_fields.py"), WORKSPACE_ROOT / "loxa-py", "duplicate field handling and general behavior"),
        Check("python", "sampling", (sys.executable, "-m", "pytest", "-q", "tests/test_behavior.py"), WORKSPACE_ROOT / "loxa-py", "sampling and lifecycle behavior"),
        Check("python", "delivery_semantics", (sys.executable, "-m", "pytest", "-q", "tests/test_delivery_semantics.py", "tests/test_httpbatch_sink.py", "tests/test_collector_ack_fixture.py", "tests/test_ingest_envelope_fixture.py"), WORKSPACE_ROOT / "loxa-py", "collector delivery and ack handling"),
        Check("python", "panic_error_safety", (sys.executable, "-m", "pytest", "-q", "tests/test_panic_safety.py"), WORKSPACE_ROOT / "loxa-py", "panic-safe error behavior"),
        Check("python", "config_precedence", (sys.executable, "-m", "pytest", "-q", "tests/test_config_loader.py"), WORKSPACE_ROOT / "loxa-py", "config and env precedence"),
        Check("python", "metrics", (sys.executable, "-m", "pytest", "-q", "tests/test_metrics_export.py"), WORKSPACE_ROOT / "loxa-py", "stats and metrics exports"),
        Check("python", "golden_fixtures", (sys.executable, "-m", "pytest", "-q", "tests/test_spec_conformance.py", "tests/test_emitted_shape_fixture.py"), WORKSPACE_ROOT / "loxa-py", "shared fixture conformance"),
        Check("python", "collector_integration", (sys.executable, "-m", "pytest", "-q", "tests/test_integration.py", "tests/test_middleware_integration.py"), WORKSPACE_ROOT / "loxa-py", "collector and middleware integration"),
        Check("python", "cortex_emitted_shape", (sys.executable, "-m", "pytest", "-q", "tests/test_emitted_shape_fixture.py"), WORKSPACE_ROOT / "loxa-py", "Cortex-consumable emitted event shape"),
        Check("python", "parity", (sys.executable, "-m", "pytest", "-q", "tests/test_parity.py"), WORKSPACE_ROOT / "loxa-py", "stable-v1 API parity vs superset manifest"),
    ],
    "rust": [
        Check("rust", "state_machine", ("cargo", "test", "-q", "--test", "state_machine"), WORKSPACE_ROOT / "loxa-rs", "core lifecycle state machine"),
        Check("rust", "canonical_fields", ("cargo", "test", "-q", "--test", "api_behavior_test"), WORKSPACE_ROOT / "loxa-rs", "canonical fields and context behavior"),
        Check("rust", "duplicate_policy", ("cargo", "test", "-q", "--test", "behavior"), WORKSPACE_ROOT / "loxa-rs", "duplicate policy and behavior"),
        Check("rust", "sampling", ("cargo", "test", "-q", "--test", "behavior"), WORKSPACE_ROOT / "loxa-rs", "sampling and lifecycle behavior"),
        Check("rust", "delivery_semantics", ("cargo", "test", "-q", "--test", "collector_ack_fixture", "--test", "httpbatch_transport", "--test", "ingest_envelope_fixture"), WORKSPACE_ROOT / "loxa-rs", "collector delivery and ack handling"),
        Check("rust", "panic_error_safety", ("cargo", "test", "-q", "--test", "smoke"), WORKSPACE_ROOT / "loxa-rs", "panic-safe error behavior"),
        Check("rust", "config_precedence", ("cargo", "test", "-q", "--test", "config_defaults"), WORKSPACE_ROOT / "loxa-rs", "config defaults and precedence"),
        Check("rust", "metrics", ("cargo", "test", "-q", "--test", "e2e"), WORKSPACE_ROOT / "loxa-rs", "runtime behavior and metric-adjacent checks"),
        Check("rust", "golden_fixtures", ("cargo", "test", "-q", "--test", "spec_contract_test", "--test", "conformance_fixtures"), WORKSPACE_ROOT / "loxa-rs", "shared fixture and manifest checks"),
        Check("rust", "collector_integration", ("cargo", "test", "-q", "--test", "integration", "--test", "e2e"), WORKSPACE_ROOT / "loxa-rs", "collector delivery and integration"),
        Check("rust", "cortex_emitted_shape", ("cargo", "test", "-q", "--test", "collector_cortex_conformance", "--test", "emitted_shape_fixture"), WORKSPACE_ROOT / "loxa-rs", "Cortex-consumable emitted event shape"),
        Check("rust", "parity", ("cargo", "test", "-q", "--test", "parity"), WORKSPACE_ROOT / "loxa-rs", "stable-v1 API parity vs superset manifest"),
    ],
}


def _all_groups() -> list[str]:
    groups = {check.group for checks in SDK_GROUPS.values() for check in checks}
    return sorted(groups)


def _run_check(check: Check, verbose: bool) -> bool:
    print(f"[{check.sdk}:{check.group}] {' '.join(check.command)}")
    print(f"  {check.description}")
    result = subprocess.run(
        list(check.command),
        cwd=check.cwd,
        capture_output=not verbose,
        text=True,
    )
    if result.returncode == 0:
        print("  PASS")
        return True
    print("  FAIL")
    if not verbose:
        output = (result.stdout or "") + (result.stderr or "")
        if output.strip():
            print(output.strip()[:5000])
    return False


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


def main() -> int:
    parser = argparse.ArgumentParser(description="Run grouped LOXA SDK conformance checks.")
    parser.add_argument("--sdk", choices=("all", "go", "python", "rust", "javascript"), default="all")
    parser.add_argument("--group", choices=("all", *_all_groups()), default="all")
    parser.add_argument("--verbose", action="store_true")
    parser.add_argument("--matrix", action="store_true", help="Print grouped conformance matrix and exit.")
    args = parser.parse_args()

    if args.matrix:
        _print_matrix()
        return 0

    checks = _selected_checks(args.sdk, args.group)
    passed = True
    for check in checks:
        passed = _run_check(check, args.verbose) and passed
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
