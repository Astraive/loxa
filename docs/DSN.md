# Loza DSN (loza://)

The `loza://` URI is the standard connection string for Loza Collector.
It resolves to HTTP/HTTPS/OTLP/gRPC/WebSocket endpoints -- it is NOT a new wire protocol.

## Format

```
loza://[host][:port]/[project]?env=<env>&service=<service>&tls=<true|false>&transport=<http|otlp|grpc>
```

## Examples

```bash
# Local development
LOZA_DSN=loza://localhost:9308/demo?env=dev&tls=false
LOZA_API_KEY=lx_sec_dev_k_xxx

# Production
LOZA_DSN=loza://collector.example.com/payments?env=prod
LOZA_API_KEY=lx_sec_prod_k_xxx

# OTLP transport for staging
LOZA_DSN=loza://loza.internal:4318/backend?env=staging&service=auth&transport=otlp
```

## Parameters

| Parameter  | Required | Default      | Description                                      |
|------------|----------|--------------|--------------------------------------------------|
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

## Security

- **Do NOT put API keys in the DSN URL.** Use `LOZA_API_KEY` environment variable separately.
- `loza://` resolves to HTTP/HTTPS, not a custom protocol.
- Userinfo in the URL (`loza://key@host/project`) is explicitly rejected.

## Environment Variable Usage

```bash
# Go SDK
export LOZA_DSN=loza://collector.example.com/my-app?env=prod
export LOZA_API_KEY=lx_sec_prod_k_xxx

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
| `loza://key@host/project`       | Do not put API keys in the URL                 |
| `loza://host/project?tls=maybe` | tls must be true, false, or auto               |
| `loza://host/project?transport=x` | transport must be http, otlp, or grpc        |
| `loza://host:0/project`         | Invalid port                                   |
| `loza://host:99999/project`     | Invalid port                                   |
