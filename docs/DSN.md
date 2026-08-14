# Loza DSN (loza://)

The `loza://` URI is the standard connection string for Loza Collector.
It resolves to HTTP/HTTPS/OTLP/gRPC/WebSocket endpoints -- it is NOT a new wire protocol.

## Format

```
loza://[<username>[:<password>]@]<host>[:port]/<collector-name>?env=<env>&service=<service>&tls=<true|false>&transport=<http|otlp|grpc>
```

The required path identifies one named Collector resource. The optional userinfo
carries either private Basic credentials (`username:password`) or a public
opaque access ID (`lx_pub_...` with no password). A DSN without userinfo is
unchanged for API-key-based configurations.

## Examples

```bash
# Local development
LOZA_DSN=loza://localhost:9308/demo?env=dev&tls=false
LOZA_API_KEY=lx_sec_dev_k_xxx

# Production
LOZA_DSN=loza://collector.example.com/payments?env=prod
LOZA_API_KEY=lx_sec_prod_k_xxx

# Private credentialed DSN (percent-encode reserved password characters)
LOZA_DSN=loza://payments-writer:s%40cret%3Avalue@collector.example.com/payments?env=prod

# Public, revocable opaque access ID with no password
LOZA_DSN=loza://lx_pub_<random>@collector.example.com/payments?env=prod

# OTLP transport for staging
LOZA_DSN=loza://loza.internal:4318/backend?env=staging&service=auth&transport=otlp
```

The private username is the Collector configured grant username and its password is the grant secret. A public username MUST be an opaque high-entropy `lx_pub_...` access ID; it is a bearer capability, not a human name. SDK parsers percent-decode private credentials. Empty private components and username `:`/whitespace are invalid; URL-reserved private password characters must be percent-encoded.

## Parameters

| Parameter  | Required | Default      | Description                                      |
|------------|----------|--------------|--------------------------------------------------|
| `username` | With userinfo | --        | Private grant username or opaque `lx_pub_...` public access ID |
| `password` | Private identity | --      | Secret for a private Collector grant; percent-encoded |
| `host`     | Yes      | --           | Collector hostname                               |
| `port`     | No       | See below    | Collector port                                   |
| `collector-name` | Yes | --           | Named Collector resource (path segment)          |
| `env`      | No       | `"default"`  | Deployment environment                           |
| `service`  | No       | `""`         | Service name                                     |
| `tls`      | No       | See below    | `true`, `false`, or `auto`                       |
| `transport`| No       | `"http"`     | `http`, `otlp`, or `grpc`                        |

## TLS Defaults

| Host                    | Default TLS |
|-------------------------|-------------|
| `localhost`             | `false`     |
| `127.0.0.1`            | `false`     |
| `::1`                  | `false`     |
| All other hosts         | `true`      |

Setting `tls=auto` preserves the computed default.

## Port Defaults

| Condition                | Default Port |
|--------------------------|-------------|
| `tls=true`               | `443`       |
| `tls=false`              | `80`        |
| `localhost` without port | `9308`      |

## Resolved URLs

A parsed `loza://` DSN resolves to credential-free, collector-scoped endpoints:

| Field        | Pattern                           | Example                                  |
|--------------|-----------------------------------|------------------------------------------|
| `BaseURL`    | `http(s)://host:port`             | `https://collector.example.com:443`      |
| `EventsURL`  | `BaseURL + /collectors/{collector}/events` | `https://collector.example.com:443/collectors/payments/events` |
| `BatchURL`   | `BaseURL + /collectors/{collector}/events/batch` | `https://collector.example.com:443/collectors/payments/events/batch` |
| `OTLPURL`    | `BaseURL + /collectors/{collector}/otlp/logs` | `https://collector.example.com:443/collectors/payments/otlp/logs` |
| `TailWSURL`  | `ws(s)://host:port/collectors/{collector}/tail` | `wss://collector.example.com:443/collectors/payments/tail` |

## Security and Authentication

- A private credentialed DSN sends `Authorization: Basic <base64(username:password)>`; SDKs use TLS by default.
- A public DSN sends `Authorization: Basic <base64(lx_pub_<random>:)>`. Its opaque public access ID is a bearer capability and MUST be redacted and protected like a secret.
- Plain HTTP with any credential is rejected by SDK configuration validation unless explicitly local (`tls=false`/insecure). The parser may parse the URI, but it must not silently enable production plaintext HTTP.
- Do not put credentials in query parameters or paths. Resolved `BaseURL`, `EventsURL`, `BatchURL`, `OTLPURL`, and `TailWSURL` never contain userinfo, passwords, or access IDs.
- Normal DSN string/debug representations must be redacted; never log private passwords or public access IDs. Use a secret manager/environment variable rather than committing a credentialed DSN.

## Credential Precedence

Credential sources are applied with this precedence:

1. Explicit code API key or Basic credentials.
2. Credentials in an explicitly supplied code DSN.
3. Environment credential sources (`LOZA_API_KEY` and userinfo in `LOZA_DSN`).

`LOZA_API_KEY` remains the highest-priority token credential. `LOZA_COLLECTOR_URL` overrides only the endpoint; it does not replace DSN-derived environment, service, or credentials. A DSN without userinfo never clears credentials configured separately.

## Environment Variable Usage

```bash
# Go SDK — existing no-userinfo/API-key form
export LOZA_DSN=loza://collector.example.com/my-app?env=prod
export LOZA_API_KEY=lx_sec_prod_k_xxx

# Private Basic-auth form; keep this value in a secret environment/secret manager
export LOZA_DSN='loza://payments-writer:s%40cret%3Avalue@collector.example.com/my-app?env=prod'

# Public opaque access ID; it is still sensitive and should be stored as a secret
export LOZA_DSN='loza://lx_pub_<random>@collector.example.com/my-app?env=prod'

# JS/TS SDK
LOZA_DSN=loza://collector.example.com/my-app?env=prod
LOZA_API_KEY=lx_sec_prod_k_xxx

# Python SDK
LOZA_DSN=loza://collector.example.com/my-app?env=prod
LOZA_API_KEY=lx_sec_prod_k_xxx
```

## Validation Errors

| Input                           | Error                                          |
|---------------------------------|------------------------------------------------|
| `http://host/project`           | Scheme must be `loza://`                       |
| `loza://`                       | Host is required                               |
| `loza:///project`               | Host is required                               |
| `loza://host`                   | Collector name path is required               |
| `loza://host/`                  | Collector name path is required               |
| `loza://host/collector`         | Valid no-userinfo DSN                          |
| `loza://writer:@host/collector` | Private username and password are required    |
| `loza://:secret@host/collector` | Private username and password are required    |
| `loza://lx_pub_<random>@host/collector` | Valid public opaque access ID DSN      |
| `loza://user@host/collector`    | Public access ID must begin `lx_pub_`          |
| `loza://host/project?transport=x` | transport must be http, otlp, or grpc        |
| `loza://host:0/project`         | Invalid port                                   |
| `loza://host:99999/project`     | Invalid port                                   |
