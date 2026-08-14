"""Parse loza:// connection URIs into resolved HTTP/HTTPS/WebSocket endpoints.

The loza:// URI is the standard connection string for Loza Collector.
It resolves to HTTP/HTTPS/OTLP/gRPC/WebSocket endpoints — it is NOT a wire protocol.

Format::

    loza://[host][:port]/[project]?env=<env>&service=<service>&tls=<true|false|auto>&transport=<http|otlp|grpc>
"""

from __future__ import annotations

import os
from dataclasses import dataclass, field
from urllib.parse import parse_qs, unquote_to_bytes, urlparse


@dataclass(frozen=True, slots=True)
class LozaDSN:
    """Parsed and resolved loza:// connection URI."""

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
    username: str = field(default="", repr=False)
    password: str = field(default="", repr=False)

    def __repr__(self) -> str:
        password = "***" if self.password else ""
        return (
            "LozaDSN("
            f"scheme={self.scheme!r}, host={self.host!r}, port={self.port!r}, "
            f"project={self.project!r}, env={self.env!r}, service={self.service!r}, "
            f"tls={self.tls!r}, transport={self.transport!r}, "
            f"base_url={self.base_url!r}, events_url={self.events_url!r}, "
            f"batch_url={self.batch_url!r}, otlp_url={self.otlp_url!r}, "
            f"tail_ws_url={self.tail_ws_url!r}, username={self.username!r}, "
            f"password={password!r})"
        )

    __str__ = __repr__


_HEX = frozenset("0123456789abcdefABCDEF")
_PASSWORD_RESERVED = frozenset(":/?#[]@!$&'()*+,;=")


def _decode_userinfo(value: str, label: str) -> str:
    for index, char in enumerate(value):
        if char == "%" and (index + 2 >= len(value) or value[index + 1] not in _HEX or value[index + 2] not in _HEX):
            raise ValueError(f"invalid Loza DSN: malformed percent-encoding in {label}")
    try:
        return unquote_to_bytes(value).decode("utf-8")
    except UnicodeDecodeError as exc:
        raise ValueError(f"invalid Loza DSN: {label} must be valid UTF-8") from exc

_LOCALHOSTS = frozenset({"localhost", "127.0.0.1", "::1"})


def _is_localhost(host: str) -> bool:
    return host in _LOCALHOSTS


def parse(raw: str) -> LozaDSN:
    """Parse a raw loza:// connection URI into a LozaDSN.

    Raises:
        ValueError: if the URI is invalid.
    """
    if not raw:
        raise ValueError("invalid Loza DSN: empty string")

    if not raw.startswith("loza://"):
        raise ValueError("invalid Loza DSN: scheme must be loza://")

    try:
        u = urlparse(raw)
    except ValueError as exc:
        raise ValueError("invalid Loza DSN: malformed URL") from exc

    username = ""
    password = ""
    if "@" in u.netloc:
        raw_userinfo = u.netloc.rsplit("@", 1)[0]
        if ":" not in raw_userinfo:
            raise ValueError("invalid Loza DSN: userinfo must include username and password")
        raw_username, raw_password = raw_userinfo.split(":", 1)
        if not raw_username or not raw_password:
            raise ValueError("invalid Loza DSN: username and password must be non-empty")
        if any(char in _PASSWORD_RESERVED for char in raw_password):
            raise ValueError("invalid Loza DSN: reserved password characters must be percent-encoded")
        username = _decode_userinfo(raw_username, "username")
        password = _decode_userinfo(raw_password, "password")
        if not username or not password:
            raise ValueError("invalid Loza DSN: username and password must be non-empty")
        if ":" in username or any(char.isspace() for char in username):
            raise ValueError("invalid Loza DSN: username must not contain ':' or whitespace")
    elif u.username is not None or u.password is not None:
        raise ValueError("invalid Loza DSN: malformed userinfo")

    host = u.hostname or ""
    if not host:
        raise ValueError("invalid Loza DSN: host is required")

    # Parse port — urlparse raises ValueError for non-numeric ports.
    try:
        port_raw = u.port
    except ValueError:
        raise ValueError("invalid Loza DSN: invalid port")

    # Project is the path segment without leading slash.
    project = u.path.lstrip("/")
    if not project:
        raise ValueError(
            "invalid Loza DSN: project path is required, e.g. loza://host/my-project"
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
                f"invalid Loza DSN: tls must be true, false, or auto, got {tls_val!r}"
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
            raise ValueError(f"invalid Loza DSN: invalid port {port_raw!r}")
        port = port_raw

    # --- Transport ----------------------------------------------------------
    transport = "http"
    transport_val = q.get("transport", [""])[0]
    if transport_val:
        if transport_val in ("http", "otlp", "grpc"):
            transport = transport_val
        else:
            raise ValueError(
                f"invalid Loza DSN: transport must be http, otlp, or grpc, got {transport_val!r}"
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

    return LozaDSN(
        scheme="loza",
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
        username=username,
        password=password,
    )


def resolve_endpoint(path: str = "", default: str = "") -> str:
    """Resolve an endpoint from LOZA_COLLECTOR_URL env var with a fallback.

    Accepts both loza:// DSN and plain http(s) URLs. If LOZA_COLLECTOR_URL
    is a loza:// DSN, it is resolved to an HTTP(S) URL first.

    Args:
        path: Suffix appended to the base URL (e.g. "/events").
        default: Fallback value when LOZA_COLLECTOR_URL is not set.

    Returns:
        The resolved endpoint URL.
    """
    base = os.getenv("LOZA_COLLECTOR_URL", "").strip()
    if base:
        if base.startswith("loza://"):
            try:
                base = parse(base).base_url
            except Exception:
                pass  # Use raw value; caller will handle the invalid URL
        return base.rstrip("/") + path
    return default
