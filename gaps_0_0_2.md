# LOXA v0.0.2 Gap Tracker (Core-First Verification)

Last updated: 2026-05-24

Architecture rule used for this pass:
- `collector/` + `cortex/` are the core behavior.
- SDKs are lightweight connectors to core.

Verification command:
- `python tests/integration/verification_matrix.py --flow matrix --json-out verification_matrix_latest.json`

Latest result:
- Matrix exit code: `0`
- Baseline core suites: passing (`collector.go`, `cortex.go`, `cli.go`, `spec.go`, `spec.py`)
- Shared SDK conformance: passing (`sdk.shared_conformance`)
- SDK category suites: passing (`sdk.go.categories`, `sdk.js.categories`, `sdk.py.categories`, `sdk.rs.categories`)

## Section Status (17.1 - 17.11)

| Section | Scope | Status |
|---|---|---|
| 17.1 | Client creation and configuration (1-25) | COMPLETE |
| 17.2 | Basic logging and event methods (26-40) | COMPLETE |
| 17.3 | Lifecycle event methods (41-60) | COMPLETE |
| 17.4 | Process/group/timer/stopwatch methods (61-80) | COMPLETE |
| 17.5 | Typed attribute helpers (81-105) | COMPLETE |
| 17.6 | Identity and domain helpers (106-125) | COMPLETE |
| 17.7 | HTTP and framework helpers (126-142) | COMPLETE |
| 17.8 | Sink/queue/flush/shutdown (143-158) | COMPLETE |
| 17.9 | Sampling and policy (159-175) | COMPLETE |
| 17.10 | Testing and conformance helpers (176-190) | COMPLETE |
| 17.11 | Collector API / CLI families (191-210) | COMPLETE |

## Fake-Green Fixes Completed In This Pass

- Fixed Python facade sink lifecycle shadowing (`pause/resume/drain/queue_size/health`) to support explicit sink targets and global logger behavior.
- Removed real stub behavior in JS/Python/Rust testkit ID generator plumbing; deterministic IDs are now behavior-tested.
- Removed Rust collector client synthetic `.example` short-circuit behavior and replaced with real request-path testing.
- Hardened collector API tests (especially Python/Rust) from surface `hasattr` checks to request/response behavior checks.
- Stabilized JS test execution in conformance and matrix via single-threaded node test concurrency.

## `/v1/*` Removal Direction (Core + Connectors)

Implemented for active connector/runtime paths:
- Collector connectors use unversioned endpoints (`/events`, `/query`, `/tail`, etc.).
- Cortex SDK clients moved to unversioned endpoints (`/reconstruct`, `/graph/...`, `/feedback/...`, `/events/...`).
- Cortex collector bridge sync paths moved to unversioned collector endpoints.
- Cortex server now serves unversioned API routes directly (no `/v1/*` runtime route dependency).
- Proto namespaces/paths moved from `loxa/v1` to `loxa/core` and regenerated.

## Evidence Artifacts

- `verification_matrix_latest.json` (full pass report)
- Matrix command used in this environment:
  - `python tests/integration/verification_matrix.py --flow matrix --json-out verification_matrix_latest.json`
  - with bundled Node runtime prepended to `PATH` so JS category tests can execute.
- Targeted runtime validation reruns:
  - `go test ./...` in `cortex/` (passes)
  - `go test ./...` in `sdks/go/src/cortex` (passes)
  - `node --test ... sdks/js/tests/collector_api_cli.test.ts` (passes)
  - `pytest sdks/py/tests/test_collector_api_cli.py` (passes)
  - `cargo test --test collector_api_cli` in `sdks/rs` (passes)

## Final Tracking State

- Remaining TODOs for 17.1-17.11: **0**
- Marked complete end-to-end from core (`collector/cortex`) through all lightweight SDK connectors.
