"""Parse loxa:// connection URIs into resolved HTTP/HTTPS/WebSocket endpoints.

The loxa:// URI is the standard connection string for Loxa Collector.
It resolves to HTTP/HTTPS/OTLP/gRPC/WebSocket endpoints — it is NOT a wire protocol.

Format::

    loxa://[host][:port]/[project]?env=<env>&service=<service>&tls=<true|false|auto>&transport=<http|otlp|grpc>
"""

from __future__ import annotations

import os
from dataclasses import dataclass
from urllib.parse import urlparse, parse_qs


@dataclass(frozen=True, slots=True)
class LoxaDSN:
    """Parsed and resolved loxa:// connection URI."""

    scheme: str
    host: str
    port: int
    project: str
    env: str
    service: str
    tls: bool
    transport: str
    base_url: str
    events_url: str
    batch_url: str
    otlp_url: str
    tail_ws_url: str


_LOCALHOSTS = frozenset({"localhost", "127.0.0.1", "::1"})


def _is_localhost(host: str) -> bool:
    return host in _LOCALHOSTS


def parse(raw: str) -> LoxaDSN:
    """Parse a raw loxa:// connection URI into a LoxaDSN.

    Raises:
        ValueError: if the URI is invalid.
    """
    if not raw:
        raise ValueError("invalid Loxa DSN: empty string")

    if not raw.startswith("loxa://"):
        raise ValueError("invalid Loxa DSN: scheme must be loxa://")

    u = urlparse(raw)

    # Reject userinfo (API keys must not be in the URL).
    if u.username:
        raise ValueError(
            "invalid Loxa DSN: do not put API keys in the URL, use LOXA_API_KEY instead"
        )

    host = u.hostname or ""
    if not host:
        raise ValueError("invalid Loxa DSN: host is required")

    # Parse port — urlparse raises ValueError for non-numeric ports.
    try:
        port_raw = u.port
    except ValueError:
        raise ValueError("invalid Loxa DSN: invalid port")

    # Project is the path segment without leading slash.
    project = u.path.lstrip("/")
    if not project:
        raise ValueError(
            "invalid Loxa DSN: project path is required, e.g. loxa://host/my-project"
        )

    q = parse_qs(u.query)

    # --- TLS default --------------------------------------------------------
    tls: bool = not _is_localhost(host)
    tls_val = q.get("tls", [""])[0]
    if tls_val:
        if tls_val == "true":
            tls = True
        elif tls_val == "false":
            tls = False
        elif tls_val == "auto":
            pass  # keep computed default
        else:
            raise ValueError(
                f"invalid Loxa DSN: tls must be true, false, or auto, got {tls_val!r}"
            )

    # --- Port default -------------------------------------------------------
    if tls:
        port = 443
    else:
        port = 80

    if _is_localhost(host) and port_raw is None:
        port = 9308

    if port_raw is not None:
        if not (1 <= port_raw <= 65535):
            raise ValueError(f"invalid Loxa DSN: invalid port {port_raw!r}")
        port = port_raw

    # --- Transport ----------------------------------------------------------
    transport = "http"
    transport_val = q.get("transport", [""])[0]
    if transport_val:
        if transport_val in ("http", "otlp", "grpc"):
            transport = transport_val
        else:
            raise ValueError(
                f"invalid Loxa DSN: transport must be http, otlp, or grpc, got {transport_val!r}"
            )

    # --- Env ----------------------------------------------------------------
    env = q.get("env", ["default"])[0]
    if not env:
        env = "default"

    service = q.get("service", [""])[0]

    # --- Build resolved URLs ------------------------------------------------
    scheme = "https" if tls else "http"
    ws_scheme = "wss" if tls else "ws"

    # IPv6 addresses must be bracketed in URLs per RFC 2732/3986.
    host_part = f"[{host}]" if ":" in host else host

    base_url = f"{scheme}://{host_part}:{port}"

    return LoxaDSN(
        scheme="loxa",
        host=host,
        port=port,
        project=project,
        env=env,
        service=service,
        tls=tls,
        transport=transport,
        base_url=base_url,
        events_url=base_url + "/events",
        batch_url=base_url + "/events/batch",
        otlp_url=base_url + "/otlp/logs",
        tail_ws_url=f"{ws_scheme}://{host_part}:{port}/tail",
    )


def resolve_endpoint(path: str = "", default: str = "") -> str:
    """Resolve an endpoint from LOXA_COLLECTOR_URL env var with a fallback.

    Accepts both loxa:// DSN and plain http(s) URLs. If LOXA_COLLECTOR_URL
    is a loxa:// DSN, it is resolved to an HTTP(S) URL first.

    Args:
        path: Suffix appended to the base URL (e.g. "/events").
        default: Fallback value when LOXA_COLLECTOR_URL is not set.

    Returns:
        The resolved endpoint URL.
    """
    base = os.getenv("LOXA_COLLECTOR_URL", "").strip()
    if base:
        if base.startswith("loxa://"):
            try:
                base = parse(base).base_url
            except Exception:
                pass  # Use raw value; caller will handle the invalid URL
        return base.rstrip("/") + path
    return default
