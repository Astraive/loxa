# Release Process

## Version Strategy

All components share the umbrella release version declared by the manifests
in `release.yaml`. The current prepared release is `0.3.4`. Individual
component releases use the tags configured in each manifest, for example:

- `v0.3.4` — full monorepo release
- `collector-v0.3.4` — collector-only release
- `py-v0.3.4` — Python SDK release
- `rs-v0.3.4` — Rust SDK release

## Release Checklist

1. All tests pass: `cd <component> && go test ./...` (or equivalent)
2. Conformance passes: `./conformance/run-all.sh`
3. Benchmarks pass: `./bench/run-all.sh`
4. CHANGELOG.md updated with release date
5. Version bumped in all config files
6. Git tag created and pushed
7. CI builds and publishes artifacts

## CI Gates

- Push to `main`: tests + conformance + doc lint
- Tag push: build + publish
- Pull request: tests + conformance + benchmarks + doc lint
