# Release Process

## Version Strategy

All components share version 1.0.0. Individual component releases use tags:
- `v1.0.0` -- full monorepo release
- `collector-v1.0.0` -- collector-only release
- `py-v1.0.0` -- Python SDK release (triggers PyPI publish)
- `rs-v1.0.0` -- Rust SDK release (triggers crates.io publish)

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
