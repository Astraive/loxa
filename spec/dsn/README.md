# loxa:// DSN Specification

This package defines the canonical `loxa://` connection URI format and the
reference Go parser implementation.

## Purpose

`loxa://` is a connection URI (like `postgres://` or `redis://`) that resolves
to HTTP/HTTPS/OTLP/gRPC/WebSocket endpoints. It is **not** a new wire protocol.

All Loxa SDKs (Go, JS, Python, Rust) must parse `loxa://` DSNs identically.
The `test-cases.json` file in this directory defines the shared test vectors
for cross-SDK validation.

## Package structure

```
spec/dsn/
  dsn.go           -- Go reference parser (package dsn)
  dsn_test.go      -- Go unit tests
  test-cases.json  -- shared test vectors for all SDKs
  README.md        -- this file
```

## Usage from the Go SDK

```go
import "github.com/astraive/loxa/spec/dsn"

d, err := dsn.Parse("loxa://localhost:8080/demo?env=dev&tls=false")
if err != nil {
    log.Fatal(err)
}
fmt.Println(d.BaseURL)    // http://localhost:8080
fmt.Println(d.EventsURL)  // http://localhost:8080/events
fmt.Println(d.TailWSURL)  // ws://localhost:8080/tail
```

## Cross-SDK validation

Each SDK should load `test-cases.json` and run its parser against all cases,
asserting that:
1. Valid inputs produce the expected field values.
2. Invalid inputs produce a parse error.

This guarantees behavioral parity across Go, JS, Python, and Rust.

## Running the Go tests

```bash
cd spec/dsn && go test -v ./...
```
