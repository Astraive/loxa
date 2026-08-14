# loza:// DSN Specification

This package defines the canonical `loza://` connection URI format and the
reference Go parser implementation.

## Purpose

`loza://` is a connection URI (like `postgres://` or `redis://`) that resolves
to HTTP/HTTPS/OTLP/gRPC/WebSocket endpoints. It is **not** a new wire protocol.

All Loza SDKs (Go, JS, Python, Rust) must parse `loza://` DSNs identically.
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
import "github.com/astraive/loza/spec/dsn"

d, err := dsn.Parse("loza://localhost:9308/demo?env=dev&tls=false")
if err != nil {
    log.Fatal(err)
}
fmt.Println(d.BaseURL)    // http://localhost:9308
fmt.Println(d.EventsURL)  // http://localhost:9308/events
fmt.Println(d.TailWSURL)  // ws://localhost:9308/tail
```

## Credentialed DSNs

Credentials are optional userinfo in PostgreSQL-style form:

```text
loza://<username>:<password>@<host>/<project>?env=prod
```

The username is the Collector key ID and the password is that key's secret.
Both values are percent-decoded by the parser; URL-reserved password characters
must be percent-encoded (for example, `s%40cret%3Avalue`). Empty credentials,
malformed escapes, usernames containing `:` or whitespace, and unencoded
reserved password characters are rejected.

Parsed credentials are exposed as `Username` and `Password`, but never appear
in `BaseURL`, `EventsURL`, `BatchURL`, `OTLPURL`, or `TailWSURL`. SDKs send
credentialed DSNs as HTTP Basic authentication and use TLS by default. Do not
place secrets in logs or general-purpose DSN strings; use a redacted
representation when displaying configuration.

For SDK configuration, explicitly supplied code credentials take precedence
over credentials in a code-supplied DSN, which take precedence over
environment DSN credentials (`LOZA_DSN`). `LOZA_API_KEY` remains the
highest-priority token credential. `LOZA_COLLECTOR_URL` changes only the
endpoint and does not override DSN-derived environment, service, or
credentials. A DSN without userinfo does not clear credentials configured
separately.

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
