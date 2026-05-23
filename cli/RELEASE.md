# Release Process

## Versioning

The CLI follows semantic versioning, aligned with the collector and cortex releases. The current stable release is **v0.0.1**.

## Pre-release Checklist

1. Run the full test suite:

```bash
cd cli
go test ./... -race -count=1
go vet ./...
```

2. Verify all commands parse correctly:

```bash
go run ./cmd/loxa --help
go run ./cmd/loxa collector --help
go run ./cmd/loxa schema --help
```

3. Run the dependency boundary conformance test:

```bash
go test ./... -run TestDependencyBoundary
```

4. Verify the defaults config file is up to date.

5. Update `CHANGELOG.md` with the new version and date.

## Build

```bash
cd cli
go build -o loxa.exe ./cmd/loxa
```

## Tagging

```bash
git tag -a cli/v0.0.1 -m "cli v0.0.1"
git push origin cli/v0.0.1
```

## Distribution

The CLI is a single binary. Distribute via:

- Direct download from GitHub releases
- `go install github.com/astraive/loxa/cli/cmd/loxa@latest`
- Build from source: `go build -o loxa.exe ./cmd/loxa`

## Post-release

1. Verify `loxa doctor` reports all checks passing.
2. Verify `loxa status` connects to a running collector.
3. Test `loxa query` and `loxa tail` against a live instance.
