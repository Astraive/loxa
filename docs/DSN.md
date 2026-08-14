# Loza DSN (loza://)

The `loza://` URI is the standard connection string for Loza Collector.
It resolves to HTTP/HTTPS/OTLP/gRPC/WebSocket endpoints -- it is NOT a new wire protocol.

## Format

```
loza://[<username>:<password>@]<host>[:port]/<project>?env=<env>&service=<service>&tls=<true|false>&transport=<http|otlp|grpc>
```

The optional PostgreSQL-style userinfo carries Collector Basic-auth credentials. A DSN without userinfo is unchanged.

## Examples

```bash
# Local development
LOZA_DSN=loza://localhost:9308/demo?env=dev&tls=false
LOZA_API_KEY=lx_sec_dev_k_xxx

# Production
LOZA_DSN=loza://collector.example.com/payments?env=prod
LOZA_API_KEY=lx_sec_prod_k_xxx

# Credentialed DSN (Basic auth; percent-encode reserved password characters)
LOZA_DSN=loza://kingest:s%40cret%3Avalue@collector.example.com/payments?env=prod

# OTLP transport for staging
LOZA_DSN=loza://loza.internal:4318/backend?env=staging&service=auth&transport=otlp
```

The username is the Collector `key_id`; the password is that key's secret. Both values are percent-decoded by SDK parsers. An empty username or password is invalid when userinfo is present, and username `:` or whitespace is not allowed. Percent-encode URL-reserved password characters (for example `@` as `%40` and `:` as `%3A`).

## Parameters

| Parameter  | Required | Default      | Description                                      |
|------------|----------|--------------|--------------------------------------------------|
| `username` | With userinfo | --        | Collector configured `key_id`                   |
| `password` | With userinfo | --        | Secret for the Collector key; percent-encoded   |
| `host`     | Yes      | --           | Collector hostname                               |
| `port`     | No       | See below    | Collector port                                   |
| `project`  | Yes      | --           | Project name (path segment)                      |
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

A parsed `loza://` DSN resolves to these endpoints:

| Field        | Pattern                           | Example                                  |
|--------------|-----------------------------------|------------------------------------------|
| `BaseURL`    | `http(s)://host:port`             | `https://collector.example.com:443`      |
| `EventsURL`  | `BaseURL + /events`               | `https://collector.example.com:443/events` |
| `BatchURL`   | `BaseURL + /events/batch`         | `https://collector.example.com:443/events/batch` |
| `OTLPURL`    | `BaseURL + /otlp/logs`            | `https://collector.example.com:443/otlp/logs` |
| `TailWSURL`  | `ws(s)://host:port/tail`          | `wss://collector.example.com:443/tail`   |

## Security and Authentication

- A credentialed DSN sends `Authorization: Basic <base64(username:password)>`; SDKs use TLS by default.
- Plain HTTP with credentials is rejected by SDK configuration validation unless explicitly local (`tls=false`/insecure). The parser may parse the URI, but it must not silently enable production plaintext HTTP.
- Do not put credentials in query parameters or paths. Resolved `BaseURL`, `EventsURL`, `BatchURL`, `OTLPURL`, and `TailWSURL` never contain userinfo or the password.
- Normal DSN string/debug representations must be redacted; never log the password or decoded credentials. Use a secret manager/environment variable rather than committing a credentialed DSN.
- Existing Bearer API-key configuration remains supported; use `LOZA_API_KEY` for the full `lx_...` token rather than placing that token in a DSN.

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

# Optional Basic-auth form; keep this value in a secret environment/secret manager
export LOZA_DSN='loza://kingest:s%40cret%3Avalue@collector.example.com/my-app?env=prod'

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
| `loza://host`                   | Project path is required                       |
| `loza://host/`                  | Project path is required                       |
| `loza://host/project`           | Valid no-userinfo DSN                          |
| `loza://kingest:@host/project` | Userinfo username and password are required    |
| `loza://:secret@host/project`  | Userinfo username and password are required    |
| `loza://key@host/project`      | Userinfo must contain `username:password`      |
| `loza://host/project?tls=maybe` | tls must be true, false, or auto               |
| `loza://host/project?transport=x` | transport must be http, otlp, or grpc        |
| `loza://host:0/project`         | Invalid port                                   |
| `loza://host:99999/project`     | Invalid port                                   |
