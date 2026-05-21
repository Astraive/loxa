# SDK Release Checklist

Use this checklist before keeping `sdks/go`, `sdks/py`, `sdks/rs`, and `sdks/js` labeled stable-v1.

## Contract

- `SDK_CONFORMANCE_CONTRACT.md` matches the current stable-v1 behavior.
- `SDK_CONFORMANCE_TEST_SUITE.md` matches the current grouped runner categories.
- `SDK_COMPLETION_MATRIX.md` matches the stable-v1 public surface and exclusions.
- `spec/docs/sdk-parity-manifest.json` is current, intentionally versioned, and mirrored into each SDK docs folder.

## Verification

- `python conformance/runner.py --sdk all --group all` passes from `spec`.
- Shared fixture validation passes.
- Each SDK passes its repo-local fast test suite.
- Each SDK passes collector delivery and emitted-shape checks.
- No parity drift exists between Go/Python/Rust/JavaScript and the superset manifest.

## CI

- SDK repo CI checks out required sibling repos for parity and fixture tests.
- `spec` CI runs codegen, mirror, fixture validation, and grouped SDK conformance.
- Failures are reported by category, not only as coarse package failures.

## Docs

- Go, Python, Rust, and JavaScript READMEs each state:
  - stable-v1 status
  - collector-first scope
  - excluded collector-owned features
  - collector integration usage
  - test/conformance entrypoints
- No repo still describes any stable-v1 SDK as alpha for the stable-v1 emission scope.

## Release Hygiene

- Changes are committed repo-by-repo with verified slices.
- Each affected repo is pushed only after its slice verification passes.
- Release notes call out any language-specific APIs that remain outside parity enforcement.
