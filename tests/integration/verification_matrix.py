#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import shutil
import socket
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request
from dataclasses import asdict, dataclass, field
from pathlib import Path
from typing import Any

WORKSPACE_ROOT = Path(__file__).resolve().parents[2]
SPEC_ROOT = WORKSPACE_ROOT / "spec"
COLLECTOR_ROOT = WORKSPACE_ROOT / "collector"
CORTEX_ROOT = WORKSPACE_ROOT / "cortex"
CLI_ROOT = WORKSPACE_ROOT / "cli"
MANIFEST_PATH = SPEC_ROOT / "docs" / "sdk-parity-manifest.json"
PYTHON = sys.executable


@dataclass
class StepResult:
    id: str
    category: str
    status: str
    command: list[str] = field(default_factory=list)
    cwd: str = ""
    duration_s: float = 0.0
    returncode: int | None = None
    stdout: str = ""
    stderr: str = ""
    details: dict[str, Any] = field(default_factory=dict)

@dataclass(frozen=True)
class CollectorRuntime:
    base_url: str
    ingest_token: str
    admin_token: str


def _tool_available(name: str) -> bool:
    return shutil.which(name) is not None


def _run(
    step_id: str,
    category: str,
    command: list[str],
    cwd: Path,
    *,
    env: dict[str, str] | None = None,
    ok_returncodes: tuple[int, ...] = (0,),
    timeout_s: float | None = None,
) -> StepResult:
    started = time.perf_counter()
    try:
        proc = subprocess.run(
            command,
            cwd=cwd,
            capture_output=True,
            text=True,
            env=env,
            timeout=timeout_s,
        )
        duration_s = time.perf_counter() - started
        status = "implemented_and_passing" if proc.returncode in ok_returncodes else "implemented_and_failing"
        return StepResult(
            id=step_id,
            category=category,
            status=status,
            command=command,
            cwd=str(cwd),
            duration_s=duration_s,
            returncode=proc.returncode,
            stdout=proc.stdout[-12000:],
            stderr=proc.stderr[-12000:],
        )
    except subprocess.TimeoutExpired as exc:
        duration_s = time.perf_counter() - started
        return StepResult(
            id=step_id,
            category=category,
            status="implemented_and_failing",
            command=command,
            cwd=str(cwd),
            duration_s=duration_s,
            returncode=None,
            stdout=((exc.stdout or "")[-12000:]),
            stderr=((exc.stderr or "")[-12000:]),
            details={"reason": f"timed out after {timeout_s}s"},
        )


def _blocked(step_id: str, category: str, reason: str, status: str = "environment_blocked") -> StepResult:
    return StepResult(id=step_id, category=category, status=status, details={"reason": reason})


def _find_free_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.bind(("127.0.0.1", 0))
        return int(sock.getsockname()[1])


def _http_request(
    method: str,
    url: str,
    *,
    body: bytes | None = None,
    headers: dict[str, str] | None = None,
    timeout: float = 10.0,
) -> tuple[int, str]:
    request = urllib.request.Request(url, data=body, method=method)
    for key, value in (headers or {}).items():
        request.add_header(key, value)
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            return response.getcode(), response.read().decode("utf-8", errors="replace")
    except urllib.error.HTTPError as exc:
        body_text = exc.read().decode("utf-8", errors="replace")
        return int(exc.code), body_text


def _has_encrypted_raw(response: str) -> bool:
    try:
        payload = json.loads(response)
    except json.JSONDecodeError:
        return False
    rows = payload.get("rows", []) if isinstance(payload, dict) else []
    return any(
        isinstance(row, dict) and isinstance(row.get("raw"), str) and row["raw"].startswith("enc:")
        for row in rows
    )

def _wait_for_http(url: str, *, timeout_s: float = 20.0) -> bool:
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        try:
            status, _ = _http_request("GET", url, timeout=2.0)
            if 200 <= status < 300:
                return True
        except Exception:
            time.sleep(0.25)
            continue
        time.sleep(0.25)
    return False


def _build_cli_config(temp_dir: Path, collector_url: str, cortex_url: str) -> Path:
    config_path = temp_dir / "loza-cli.yaml"
    config_path.write_text(
        "\n".join(
            [
                f"collector_repo_path: {COLLECTOR_ROOT.as_posix()}",
                f"cortex_repo_path: {CORTEX_ROOT.as_posix()}",
                f"spec_repo_path: {SPEC_ROOT.as_posix()}",
                f"collector_url: {collector_url}",
                "duckdb_path: loza.db",
                "spool_dir: loza-spool",
                "spool_file: spool.ndjson",
                "dlq_path: loza-dlq.ndjson",
                "cortex:",
                f"  url: {cortex_url}",
            ]
        ),
        encoding="utf-8",
    )
    return config_path


def _start_collector(
    temp_dir: Path, port: int
) -> tuple[subprocess.Popen[str] | None, list[StepResult], CollectorRuntime | None]:
    results: list[StepResult] = []
    if not _tool_available("go"):
        results.append(_blocked("collector.build", "collector_runtime", "go is required to build the collector"))
        return None, results, None

    binary = temp_dir / ("loza-collector.exe" if os.name == "nt" else "loza-collector")
    build = _run(
        "collector.build",
        "collector_runtime",
        ["go", "build", "-o", str(binary), "./cmd/loza-collector"],
        COLLECTOR_ROOT,
    )
    results.append(build)
    if build.status != "implemented_and_passing":
        return None, results, None

    auth_server_secret = "collector-integration-auth-server-secret"
    ingest_secret = "collector-integration-ingest-secret"
    admin_secret = "collector-integration-admin-secret"
    storage_encryption_key = "collector-integration-storage-encryption-key"
    config_path = temp_dir / "collector.yaml"
    config_path.write_text(
        "\n".join(
            [
                'version: "1.0"',
                "auth:",
                "  enabled: true",
                '  header: "Authorization"',
                "  server_secret: ${COLLECTOR_AUTH_SERVER_SECRET}",
                "  cache_ttl: 1m",
                "  negative_cache_ttl: 1s",
                "  keys:",
                "    - name: integration-ingest",
                "      key_id: kingest",
                "      secret_env: COLLECTOR_INGEST_KEY_SECRET",
                "      kind: sec",
                "      roles: [collector_ingest_server]",
                "    - name: integration-admin",
                "      key_id: kadmin",
                "      secret_env: COLLECTOR_ADMIN_KEY_SECRET",
                "      kind: sec",
                "      roles: [project_admin]",
                "storage:",
                "  encryption_key_env: LOZA_STORAGE_ENCRYPTION_KEY",
                "duckdb:",
                f"  path: {json.dumps(str(temp_dir / 'collector.duckdb'))}",
            ]
        )
        + "\n",
        encoding="utf-8",
    )

    env = os.environ.copy()
    env.update(
        {
            "COLLECTOR_AUTH_SERVER_SECRET": auth_server_secret,
            "COLLECTOR_INGEST_KEY_SECRET": ingest_secret,
            "COLLECTOR_ADMIN_KEY_SECRET": admin_secret,
            "LOZA_STORAGE_ENCRYPTION_KEY": storage_encryption_key,
        }
    )
    stdout_file = temp_dir / "collector.stdout.log"
    stderr_file = temp_dir / "collector.stderr.log"
    stdout_handle = stdout_file.open("w", encoding="utf-8")
    stderr_handle = stderr_file.open("w", encoding="utf-8")
    proc = subprocess.Popen(
        [str(binary), "run", "-c", str(config_path), "--addr", f"127.0.0.1:{port}"],
        cwd=COLLECTOR_ROOT,
        env=env,
        stdout=stdout_handle,
        stderr=stderr_handle,
        text=True,
    )
    stdout_handle.close()
    stderr_handle.close()
    return (
        proc,
        results,
        CollectorRuntime(
            base_url=f"http://127.0.0.1:{port}",
            ingest_token=f"lx_sec_live_kingest_{ingest_secret}",
            admin_token=f"lx_sec_live_kadmin_{admin_secret}",
        ),
    )


def _stop_process(proc: subprocess.Popen[str] | None) -> None:
    if proc is None:
        return
    if proc.poll() is None:
        proc.terminate()
        try:
            proc.wait(timeout=10)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait(timeout=5)


def run_collector_smoke() -> list[StepResult]:
    results: list[StepResult] = []
    with tempfile.TemporaryDirectory(prefix="loza-collector-smoke-") as raw_dir:
        temp_dir = Path(raw_dir)
        port = _find_free_port()
        proc, startup_results, collector = _start_collector(temp_dir, port)
        results.extend(startup_results)
        if proc is None or collector is None:
            return results
        try:
            started = time.perf_counter()
            healthy = _wait_for_http(collector.base_url + "/health")
            results.append(
                StepResult(
                    id="collector.health",
                    category="collector_runtime",
                    status="implemented_and_passing" if healthy else "implemented_and_failing",
                    duration_s=time.perf_counter() - started,
                    details={"url": collector.base_url + "/health"},
                )
            )
            if not healthy:
                return results

            version_status, version_body = _http_request("GET", collector.base_url + "/version")
            results.append(
                StepResult(
                    id="collector.version",
                    category="collector_runtime",
                    status="implemented_and_passing" if version_status == 200 else "implemented_and_failing",
                    duration_s=0.0,
                    details={"response": version_body},
                )
            )

            unauthorized_status, unauthorized_body = _http_request("GET", collector.base_url + "/status")
            unauthorized_ok = (
                unauthorized_status == 401
                and unauthorized_body.strip() == '{"error":"unauthorized"}'
            )
            results.append(
                StepResult(
                    id="collector.unauthenticated_request",
                    category="collector_runtime",
                    status="implemented_and_passing" if unauthorized_ok else "implemented_and_failing",
                    duration_s=0.0,
                    details={"response": unauthorized_body},
                )
            )

            marker = f"collector-smoke-{int(time.time())}"
            payload = {
                "schema_version": "v1",
                "event_version": "v1",
                "event_id": f"evt_{marker}",
                "timestamp": "2026-05-23T00:00:00Z",
                "service": "collector-smoke",
                "event": "collector.smoke",
                "kind": "cli",
                "level": "info",
                "outcome": "success",
                "event_state": "finished",
                "attrs": {"test_marker": marker},
            }
            ingest_status, ingest_body = _http_request(
                "POST",
                collector.base_url + "/events",
                body=json.dumps(payload).encode("utf-8"),
                headers={
                    "Authorization": f"Bearer {collector.ingest_token}",
                    "Content-Type": "application/json",
                },
            )
            results.append(
                StepResult(
                    id="collector.ingest",
                    category="collector_runtime",
                    status="implemented_and_passing" if ingest_status in (200, 202) else "implemented_and_failing",
                    details={"response": ingest_body, "marker": marker},
                )
            )

            time.sleep(1.0)
            query_status, query_body = _http_request(
                "POST",
                collector.base_url + "/query",
                body=json.dumps({"query": "SELECT raw FROM events ORDER BY timestamp DESC LIMIT 20"}).encode("utf-8"),
                headers={
                    "Authorization": f"Bearer {collector.admin_token}",
                    "Content-Type": "application/json",
                },
            )
            query_ok = query_status == 200 and _has_encrypted_raw(query_body)
            results.append(
                StepResult(
                    id="collector.query",
                    category="collector_runtime",
                    status="implemented_and_passing" if query_ok else "implemented_and_failing",
                    details={"response": query_body, "marker": marker},
                )
            )
        except urllib.error.URLError as exc:
            results.append(
                StepResult(
                    id="collector.http",
                    category="collector_runtime",
                    status="implemented_and_failing",
                    details={"error": str(exc)},
                )
            )
        finally:
            _stop_process(proc)
    return results


def run_cli_flow() -> list[StepResult]:
    results: list[StepResult] = []
    if not _tool_available("go"):
        return [_blocked("cli.build", "cli_runtime", "go is required to build the CLI")]

    with tempfile.TemporaryDirectory(prefix="loza-cli-flow-") as raw_dir:
        temp_dir = Path(raw_dir)
        port = _find_free_port()
        collector_proc, startup_results, collector = _start_collector(temp_dir, port)
        results.extend(startup_results)
        if collector_proc is None or collector is None:
            return results

        cli_binary = temp_dir / ("loza.exe" if os.name == "nt" else "loza")
        build = _run("cli.build", "cli_runtime", ["go", "build", "-o", str(cli_binary), "./cmd/loza"], CLI_ROOT)
        results.append(build)
        if build.status != "implemented_and_passing":
            _stop_process(collector_proc)
            return results

        config_path = _build_cli_config(temp_dir, collector.base_url, "http://127.0.0.1:9312")
        env = os.environ.copy()
        env["LOZA_CLI_CONFIG"] = str(config_path)
        env["LOZA_API_KEY"] = collector.admin_token

        if not _wait_for_http(collector.base_url + "/health"):
            results.append(_blocked("cli.collector", "cli_runtime", "collector did not become healthy", "implemented_and_failing"))
            _stop_process(collector_proc)
            return results

        results.append(_run("cli.schema_validate", "cli_runtime", [str(cli_binary), "schema", "validate"], CLI_ROOT, env=env))
        results.append(_run("cli.status", "cli_runtime", [str(cli_binary), "--output=json", "status"], CLI_ROOT, env=env))

        marker = f"cli-flow-{int(time.time())}"
        attrs = json.dumps({"test_marker": marker})
        env["LOZA_API_KEY"] = collector.ingest_token
        emit = _run(
            "cli.emit",
            "cli_runtime",
            [str(cli_binary), "emit", "sample", "--service", "cli-flow", "--event", "cli.flow", "--attrs", attrs],
            CLI_ROOT,
            env=env,
        )
        results.append(emit)

        time.sleep(1.0)
        query_status, query_body = _http_request(
            "POST",
            collector.base_url + "/query",
            body=json.dumps({"query": "SELECT raw FROM events ORDER BY timestamp DESC LIMIT 20"}).encode("utf-8"),
            headers={
                "Authorization": f"Bearer {collector.admin_token}",
                "Content-Type": "application/json",
            },
        )
        results.append(
            StepResult(
                id="cli.query_roundtrip",
                category="cli_runtime",
                status="implemented_and_passing" if query_status == 200 and _has_encrypted_raw(query_body) else "implemented_and_failing",
                details={"marker": marker, "response": query_body},
            )
        )

        _stop_process(collector_proc)
    return results


def run_cortex_full_stack() -> list[StepResult]:
    results: list[StepResult] = []
    if not _tool_available("docker"):
        return [_blocked("cortex.docker", "cortex_runtime", "docker is required for the cortex full-stack flow")]

    compose_version = _run(
        "cortex.compose.version",
        "cortex_runtime",
        ["docker", "compose", "version"],
        CORTEX_ROOT,
        timeout_s=30,
    )
    results.append(compose_version)
    if compose_version.status != "implemented_and_passing":
        results[-1].status = "environment_blocked"
        results[-1].details["reason"] = "docker compose is unavailable"
        return results

    compose_env = os.environ.copy()
    compose_env.setdefault("POSTGRES_PASSWORD", "loza-integration-postgres-password")
    compose_env.setdefault("CORTEX_API_KEYS", "integration:loza-integration-cortex-api-key:admin")

    up = _run(
        "cortex.compose.up",
        "cortex_runtime",
        ["docker", "compose", "-f", "configs/docker-compose.yml", "up", "--build", "-d", "--wait"],
        CORTEX_ROOT,
        env=compose_env,
        timeout_s=120,
    )
    compose_err = (up.stderr or "").lower()
    if up.status != "implemented_and_passing" and (
        "error during connect" in compose_err
        or "dockerdesktoplinuxengine" in compose_err
        or "cannot find the file specified" in compose_err
        or "daemon" in compose_err
    ):
        up.status = "environment_blocked"
        up.details["reason"] = "docker engine is unavailable"
    results.append(up)
    if up.status != "implemented_and_passing":
        if up.status != "environment_blocked":
            results.append(
                _run(
                    "cortex.compose.down",
                    "cortex_runtime",
                    ["docker", "compose", "-f", "configs/docker-compose.yml", "down", "--remove-orphans"],
                    CORTEX_ROOT,
                    env=compose_env,
                    ok_returncodes=(0,),
                    timeout_s=60,
                )
            )
        return results

    try:
        health_ok = _wait_for_http("http://127.0.0.1:9312/healthz", timeout_s=30.0)
        ready_ok = _wait_for_http("http://127.0.0.1:9312/readyz", timeout_s=30.0)
        results.append(
            StepResult(
                id="cortex.healthz",
                category="cortex_runtime",
                status="implemented_and_passing" if health_ok else "implemented_and_failing",
            )
        )
        results.append(
            StepResult(
                id="cortex.readyz",
                category="cortex_runtime",
                status="implemented_and_passing" if ready_ok else "implemented_and_failing",
            )
        )
        if _tool_available("go"):
            results.append(
                _run(
                    "cortex.grpc_provenance",
                    "cortex_runtime",
                    ["go", "test", "./internal/api", "-count=1", "-run", "TestGRPCServerProvenanceIsGrpc"],
                    CORTEX_ROOT,
                    timeout_s=180,
                )
            )
            results.append(
                _run(
                    "collector.cortex_bridge",
                    "cortex_runtime",
                    ["go", "test", "./cmd/loza-collector", "-count=1", "-run", "TestCortexBridge"],
                    COLLECTOR_ROOT,
                    timeout_s=180,
                )
            )
        else:
            results.append(_blocked("cortex.go_tests", "cortex_runtime", "go is unavailable for bridge/provenance tests"))
    finally:
        results.append(
            _run(
                "cortex.compose.down",
                "cortex_runtime",
                ["docker", "compose", "-f", "configs/docker-compose.yml", "down", "--remove-orphans"],
                CORTEX_ROOT,
                env=compose_env,
                ok_returncodes=(0,),
                timeout_s=60,
            )
        )
    return results


def run_baseline_product_suites() -> list[StepResult]:
    results: list[StepResult] = []
    if _tool_available("go"):
        results.append(_run("spec.go", "baseline", ["go", "test", "./..."], SPEC_ROOT, timeout_s=300))
        results.append(_run("collector.go", "baseline", ["go", "test", "./..."], COLLECTOR_ROOT, timeout_s=300))
        results.append(_run("cortex.go", "baseline", ["go", "test", "./..."], CORTEX_ROOT, timeout_s=300))
        results.append(_run("cli.go", "baseline", ["go", "test", "./..."], CLI_ROOT, timeout_s=300))
    else:
        results.extend(
            [
                _blocked("spec.go", "baseline", "go is unavailable"),
                _blocked("collector.go", "baseline", "go is unavailable"),
                _blocked("cortex.go", "baseline", "go is unavailable"),
                _blocked("cli.go", "baseline", "go is unavailable"),
            ]
        )
    results.append(_run("spec.py", "baseline", [PYTHON, "-m", "pytest", "tests", "-q"], SPEC_ROOT, timeout_s=180))
    return results


def run_shared_sdk_conformance() -> list[StepResult]:
    results: list[StepResult] = []
    if not _tool_available("node"):
        return [_blocked("sdk.shared_conformance", "sdk_conformance", "node is unavailable")]

    with tempfile.TemporaryDirectory(prefix="loza-shared-conformance-") as raw_dir:
        temp_dir = Path(raw_dir)
        collector_port = _find_free_port()
        collector_proc, startup_results, collector = _start_collector(temp_dir, collector_port)
        results.extend(startup_results)
        if collector_proc is None or collector is None:
            return results
        try:
            if not _wait_for_http(collector.base_url + "/health", timeout_s=30):
                results.append(_blocked("sdk.shared_conformance", "sdk_conformance", "collector did not become healthy", "implemented_and_failing"))
                return results

            env = os.environ.copy()
            env["LOZA_TEST_COLLECTOR_URL"] = collector.base_url
            env["LOZA_API_KEY"] = collector.ingest_token
            env["LOZA_COLLECTOR_API_KEY"] = collector.ingest_token
            env["LOZA_TEST_COLLECTOR_ADMIN_KEY"] = collector.admin_token

            if _tool_available("cargo"):
                results.append(
                    _run(
                        "sdk.shared_conformance",
                        "sdk_conformance",
                        [PYTHON, "conformance/runner.py", "--sdk", "all", "--group", "all", "--json"],
                        SPEC_ROOT,
                        env=env,
                        timeout_s=600,
                    )
                )
            else:
                for sdk_name in ("javascript", "go", "python"):
                    results.append(
                        _run(
                            f"sdk.shared_conformance.{sdk_name}",
                            "sdk_conformance",
                            [PYTHON, "conformance/runner.py", "--sdk", sdk_name, "--group", "all", "--json"],
                            SPEC_ROOT,
                            env=env,
                            timeout_s=600,
                        )
                    )
            results.append(
                _run(
                    "sdk.go.collector_e2e",
                    "sdk_conformance",
                    ["go", "test", ".", "-count=1"],
                    WORKSPACE_ROOT / "sdks" / "go" / "tests" / "e2e",
                    env=env,
                    timeout_s=180,
                )
                if _tool_available("go")
                else _blocked("sdk.go.collector_e2e", "sdk_conformance", "go is unavailable")
            )
            results.append(
                _run(
                    "sdk.js.collector_e2e",
                    "sdk_conformance",
                    ["npm.cmd" if os.name == "nt" else "npm", "test"],
                    WORKSPACE_ROOT / "sdks" / "js",
                    env=env,
                    timeout_s=180,
                )
                if _tool_available("npm")
                else _blocked("sdk.js.collector_e2e", "sdk_conformance", "npm is unavailable")
            )
            results.append(
                _run(
                    "sdk.py.collector_e2e",
                    "sdk_conformance",
                    [PYTHON, "-m", "pytest", "-q", "tests/test_e2e_live.py"],
                    WORKSPACE_ROOT / "sdks" / "py",
                    env=env,
                    timeout_s=180,
                )
            )
            results.append(
                _run(
                    "sdk.rs.collector_e2e",
                    "sdk_conformance",
                    ["cargo", "test", "--test", "collector_e2e", "--", "--include-ignored"],
                    WORKSPACE_ROOT / "sdks" / "rs",
                    env=env,
                    timeout_s=300,
                )
                if _tool_available("cargo")
                else _blocked("sdk.rs.collector_e2e", "sdk_conformance", "cargo is unavailable")
            )
        finally:
            _stop_process(collector_proc)
    return results


def run_sdk_category_suites() -> list[StepResult]:
    return [
        _run(
            "sdk.go.categories",
            "sdk_categories",
            [
                "go",
                "test",
                "./tests/conformance",
                "-run",
                "Test(ClientCreationAndConfiguration|BasicLoggingAndEventFacades|LifecycleAndTimingHelpers|TypedAttrsAndDomainHelpers|SamplingRedactionTestkitAndClients)$",
                "-count=1",
            ],
            WORKSPACE_ROOT / "sdks" / "go",
            timeout_s=180,
        )
        if _tool_available("go")
        else _blocked("sdk.go.categories", "sdk_categories", "go is unavailable"),
        _run(
            "sdk.js.categories",
            "sdk_categories",
            [
                "node",
                "--test",
                "--test-concurrency=1",
                "--experimental-strip-types",
                "tests/client_creation.test.ts",
                "tests/basic_logging_events.test.ts",
                "tests/lifecycle_event.test.ts",
                "tests/process_group_timer_stopwatch.test.ts",
                "tests/typed_attribute_helpers.test.ts",
                "tests/identity_domain_helpers.test.ts",
                "tests/sink_queue_flush_shutdown.test.ts",
                "tests/sampling_policy.test.ts",
                "tests/testing_conformance.test.ts",
                "tests/collector_api_cli.test.ts",
            ],
            WORKSPACE_ROOT / "sdks" / "js",
            timeout_s=180,
        )
        if _tool_available("node")
        else _blocked("sdk.js.categories", "sdk_categories", "node is unavailable"),
        _run(
            "sdk.py.categories",
            "sdk_categories",
            [
                PYTHON,
                "-m",
                "pytest",
                "tests/test_client_creation.py",
                "tests/test_basic_logging_events.py",
                "tests/test_lifecycle_event.py",
                "tests/test_process_group_timer_stopwatch.py",
                "tests/test_typed_attribute_helpers.py",
                "tests/test_identity_domain_helpers.py",
                "tests/test_sink_queue_flush_shutdown.py",
                "tests/test_sampling_policy.py",
                "tests/test_testing_conformance.py",
                "tests/test_collector_api_cli.py",
                "-q",
            ],
            WORKSPACE_ROOT / "sdks" / "py",
            timeout_s=300,
        ),
        _run(
            "sdk.rs.categories",
            "sdk_categories",
            [
                "cargo",
                "test",
                "-q",
                "--test",
                "client_creation",
                "--test",
                "basic_logging_events",
                "--test",
                "lifecycle_event",
                "--test",
                "process_group_timer_stopwatch",
                "--test",
                "typed_attribute_helpers",
                "--test",
                "identity_domain_helpers",
                "--test",
                "sink_queue_flush_shutdown",
                "--test",
                "sampling_policy",
                "--test",
                "testing_conformance",
                "--test",
                "collector_api_cli",
            ],
            WORKSPACE_ROOT / "sdks" / "rs",
            timeout_s=240,
        )
        if _tool_available("cargo")
        else _blocked("sdk.rs.categories", "sdk_categories", "cargo is unavailable"),
    ]


def _family_status_from_results(result_map: dict[str, StepResult], sdk: str, family: str) -> dict[str, Any]:
    sdk_key_map = {
        "go": "go",
        "golang": "go",
        "js": "js",
        "javascript": "js",
        "typescript": "js",
        "py": "py",
        "python": "py",
        "rs": "rs",
        "rust": "rs",
    }
    normalized_sdk = sdk_key_map.get(sdk.lower(), sdk.lower())
    mapping = {
        "client_creation_and_configuration": f"sdk.{normalized_sdk}.categories",
        "basic_logging_and_events": f"sdk.{normalized_sdk}.categories",
        "lifecycle": f"sdk.{normalized_sdk}.categories",
        "process_group_timer_stopwatch": f"sdk.{normalized_sdk}.categories",
        "typed_attributes": f"sdk.{normalized_sdk}.categories",
        "identity_and_domain_helpers": f"sdk.{normalized_sdk}.categories",
        "http_and_framework_helpers": f"sdk.{normalized_sdk}.categories",
        "sink_queue_flush_shutdown": f"sdk.{normalized_sdk}.categories",
        "sampling_and_policy": f"sdk.{normalized_sdk}.categories",
        "testing_and_conformance": f"sdk.{normalized_sdk}.categories",
        "collector_api_and_cli": f"sdk.{normalized_sdk}.categories",
    }
    step_id = mapping.get(family)
    if step_id is None:
        return {
            "status": "documented_but_not_behaviorally_verified",
            "reason": "no category suite mapping yet",
        }
    result = result_map.get(step_id)
    if result is None:
        return {
            "status": "documented_but_not_behaviorally_verified",
            "reason": f"missing suite result {step_id}",
        }
    if result.status == "implemented_and_passing":
        return {"status": "implemented_and_passing", "evidence": step_id}
    if result.status == "environment_blocked":
        return {"status": "environment_blocked", "reason": result.details.get("reason", "")}
    return {"status": "implemented_and_failing", "evidence": step_id}


def build_matrix(results: list[StepResult]) -> dict[str, Any]:
    manifest = json.loads(MANIFEST_PATH.read_text(encoding="utf-8"))
    result_map = {result.id: result for result in results}
    family_matrix: dict[str, dict[str, Any]] = {}
    for sdk in manifest["sdks"]:
        family_matrix[sdk] = {}
        for family, methods in manifest["method_catalog"].items():
            family_matrix[sdk][family] = {
                "methods": methods,
                **_family_status_from_results(result_map, sdk, family),
            }
    return {
        "generated_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "workspace_root": str(WORKSPACE_ROOT),
        "manifest": {
            "path": str(MANIFEST_PATH),
            "version": manifest["version"],
            "stability": manifest["stability"],
        },
        "results": [asdict(result) for result in results],
        "sdk_method_families": family_matrix,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description="Run platform-safe LOZA verification flows and emit a machine-readable matrix.")
    parser.add_argument(
        "--flow",
        choices=("collector_smoke", "cli_flow", "cortex_full_stack", "sdk_collector_e2e", "matrix"),
        default="matrix",
    )
    parser.add_argument("--json-out", help="Write JSON output to this path.")
    args = parser.parse_args()

    if args.flow == "collector_smoke":
        results = run_collector_smoke()
        payload: dict[str, Any] = {"flow": args.flow, "results": [asdict(result) for result in results]}
    elif args.flow == "cli_flow":
        results = run_cli_flow()
        payload = {"flow": args.flow, "results": [asdict(result) for result in results]}
    elif args.flow == "cortex_full_stack":
        results = run_cortex_full_stack()
        payload = {"flow": args.flow, "results": [asdict(result) for result in results]}
    elif args.flow == "sdk_collector_e2e":
        results = run_shared_sdk_conformance()
        payload = {"flow": args.flow, "results": [asdict(result) for result in results]}
    else:
        results = []
        results.extend(run_baseline_product_suites())
        results.extend(run_shared_sdk_conformance())
        results.extend(run_sdk_category_suites())
        results.extend(run_collector_smoke())
        results.extend(run_cli_flow())
        results.extend(run_cortex_full_stack())
        payload = build_matrix(results)

    if args.json_out:
        Path(args.json_out).write_text(json.dumps(payload, indent=2), encoding="utf-8")
    print(json.dumps(payload, indent=2))

    statuses = [result["status"] if isinstance(result, dict) else result.status for result in payload.get("results", [])]
    if args.flow == "matrix":
        statuses = [result["status"] for result in payload["results"]]
    failed = any(status == "implemented_and_failing" for status in statuses)
    return 1 if failed else 0


if __name__ == "__main__":
    raise SystemExit(main())
