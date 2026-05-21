# LOXA Authentication Specification

## Overview

LOXA provides unified authentication across all components using multiple methods:
- **API Key**: Simple header-based authentication
- **JWT**: Token-based authentication with claims support
- **mTLS**: Mutual TLS with client certificate validation

## Auth Modes

| Mode | Description | Use Case |
|------|-------------|---------|
| `none` | No authentication | Development/local |
| `api_key` | API key in header | Service-to-service |
| `jwt` | JWT bearer tokens | User/SSO authentication |
| `mtls` | Mutual TLS | High-security environments |

## Configuration

```yaml
auth:
  enabled: true              # Enable/disable auth (default: false)
  mode: "api_key"           # Auth mode: none, api_key, jwt, mtls
  api_key_header: "X-API-Key" # Header for API key
  api_keys:               # List of API keys
    - name: "service-a"
      key: "loxa_xxx_xxx"
      role: "writer"       # reader, writer, admin
    - name: "admin"
      key: "loxa_xxx_xxx"
      role: "admin"
  jwt_secret: ""          # JWT verification key (HMAC or RSA/ECDSA pubkey)
  jwt_issuer: ""          # Expected JWT issuer (optional)
  jwt_audience: []        # Expected JWT audience (optional)
  required_version: "v1"   # Required API version for auth
```

## API Key Format

- Prefix: `loxa_`
- Format: `{random_32_chars}` 
- Example: `loxa_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6`

## Roles

| Role | Permissions |
|------|-------------|
| `reader` | GET requests only |
| `writer` | GET + POST to ingest/feedback endpoints |
| `admin` | All requests including config changes |

## HTTP Headers

### Request (Client → Server)
```
X-API-Key: loxa_xxxxxxxxxxxxxxxxxxxxxxxxxxxxx
Authorization: Bearer <jwt_token>
```

### Response (Server → Client)
```
HTTP/401 Unauthorized
X-Auth-Failure-Reason: <reason>
X-Auth-Failure-Code: <code>
```

## Failure Codes

| Code | Reason | Description |
|------|--------|--------------|
| `missing` | No credentials provided | Auth header required |
| `invalid` | Invalid credentials | Key/token malformed |
| `expired` | Expired credentials | Token expired |
| `revoked` | Revoked credentials | Key was revoked |
| `rate_limited`| Rate limited | Too many requests |
| `unauthorized_role` | Role insufficient | Permission denied |

## Implementation Requirements

### Security
- Constant-time comparison for API keys (prevent timing attacks)
- Secure key storage (env vars, secrets managers)
- Key rotation support
- Audit logging for all auth events

### Standardization
- Shared auth package across all languages
- Consistent config schema
- Compatible JWT claims structure
- Standard error responses

## JWT Claims Structure

```json
{
  "sub": "user-id",
  "name": "John Doe",
  "role": "writer",
  "iss": "loxa",
  "aud": ["loxa-cortex", "loxa-collector"],
  "exp": 1699999999,
  "iat": 1699999999
}
```

## Rate Limiting (Optional)

Per-API-key rate limiting can be enabled alongside authentication:

```yaml
rate_limit:
  enabled: true
  per_api_key_rpm: 1000    # requests per minute
  per_ip_rpm: 100         # requests per minute (unauthenticated)
```

## Service Integration

### loxa-collector
- Uses auth for all write endpoints
- Fanout mode pushes to Cortex with auth

### loxa-cortex
- Uses auth for event ingestion
- Uses auth for incident reconstruction
- Uses auth for feedback recording

### loxa-cli
- Uses auth config for all commands
- Stores credentials in config or env vars

### SDKs
- Python: `LoxaClient(api_key="loxa_...")`
- Go: `loxa.NewClient(loxa.WithAPIKey("loxa_..."))`
- Rust: `LoxaClient::new().api_key("loxa_...")`