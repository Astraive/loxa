from __future__ import annotations

import base64
import json
import socket
import time
from datetime import datetime, timezone
from email.utils import parsedate_to_datetime
from ipaddress import ip_address
from os import getenv
from typing import Any, Iterable
from urllib.parse import urlencode, urlparse
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen
from ..generated.spec_contract import (
    CollectorResponse,
    LOZA_INGEST_API_VERSION,
    build_ingest_envelope,
    parse_collector_response,
    validate_event_payload,
)
from ..version import SDK_VERSION
from .dsn import is_public_dsn_username


class InstrumentedHTTPClient:
    def open(self, request: Request, timeout: float = 5.0):
        return urlopen(request, timeout=timeout)


def _decode_event(encoded: str) -> dict:
    payload = json.loads(encoded)
    if not isinstance(payload, dict):
        raise ValueError("encoded events must decode to JSON objects")
    validate_event_payload(payload, strict=False)
    return payload


def _build_ingest_body(encoded_events: Iterable[str], sdk_name: str, sdk_version: str, service: str) -> bytes:
    events = [_decode_event(encoded) for encoded in encoded_events]
    if not events:
        raise ValueError("collector batch must contain at least one event")
    envelope = build_ingest_envelope(events, sdk_name, sdk_version, service)
    _validate_ingest_envelope(envelope)
    return json.dumps(envelope, separators=(",", ":")).encode("utf-8")


def _parse_collector_response_body(raw: bytes) -> tuple[dict, CollectorResponse]:
    payload: dict = {}
    if raw:
        payload_obj = json.loads(raw.decode("utf-8"))
        if not isinstance(payload_obj, dict):
            raise ValueError("collector response must be a JSON object")
        payload = payload_obj
    return payload, parse_collector_response(payload)


def _collector_response_summary(response: CollectorResponse) -> str:
    return (
        response.error
        or response.reason
        or f"accepted={response.accepted} rejected={response.rejected} invalid={response.invalid}"
    )


def _collector_response_outcome(status: int, response: CollectorResponse) -> tuple[str, str]:
    retryable, reason = response.retryable_error()
    if retryable:
        return "retryable", reason or _collector_response_summary(response)
    if status in {429, 503}:
        return "retryable", reason or _collector_response_summary(response)
    failed, reason = response.permanent_failure()
    if failed or status >= 300:
        return "permanent", reason or _collector_response_summary(response)
    return "success", ""


def _validate_ingest_envelope(payload: dict) -> None:
    errors: list[str] = []
    if payload.get("api_version") != LOZA_INGEST_API_VERSION:
        errors.append('field "api_version" must match the ingest contract')
    source = payload.get("source")
    if not isinstance(source, dict):
        errors.append('field "source" must be a JSON object')
    else:
        for key in ("sdk", "version", "service"):
            value = source.get(key)
            if not isinstance(value, str) or not value.strip():
                errors.append(f'field "source.{key}" must be a non-empty string')
    events = payload.get("events")
    if not isinstance(events, list) or not events:
        errors.append('field "events" must be a non-empty array')
    elif any(not isinstance(item, dict) for item in events):
        errors.append('field "events" must contain JSON objects only')
    if errors:
        raise ValueError("; ".join(errors))


def _parse_retry_after(value: str | None) -> float | None:
    if not value:
        return None
    value = value.strip()
    if not value:
        return None
    try:
        return float(int(value))
    except ValueError:
        pass
    try:
        parsed = parsedate_to_datetime(value)
    except (TypeError, ValueError, IndexError):
        return None
    if parsed.tzinfo is None:
        parsed = parsed.replace(tzinfo=timezone.utc)
    delay = (parsed - datetime.now(timezone.utc)).total_seconds()
    return delay if delay > 0 else None


def _retry_delay(attempt: int, base_delay: float, max_delay: float, retry_after: float | None = None) -> float:
    if retry_after is not None:
        return min(max_delay, max(0.0, retry_after))
    return min(max_delay, base_delay * (2**attempt))


class CollectorClient:
    def __init__(
        self,
        endpoint: str,
        api_key: str = "",
        auth_header: str = "Authorization",
        timeout: float = 2.0,
        retries: int = 2,
        sdk_name: str = "loza-py",
        sdk_version: str = SDK_VERSION,
        service: str = "",
        username: str = "",
        password: str = "",
    ) -> None:
        _validate_collector_endpoint(endpoint)
        _validate_basic_auth_endpoint(endpoint, username, password)
        self.endpoint = endpoint
        self.api_key = api_key
        self.auth_header = auth_header
        self.timeout = timeout
        self.retries = retries
        self.sdk_name = sdk_name
        self.sdk_version = sdk_version
        self.service = service
        self.username = username
        self.password = password

    def _auth_headers(self) -> dict[str, str]:
        if self.api_key:
            if self.auth_header.lower() == "authorization":
                return {self.auth_header: f"Bearer {self.api_key}"}
            return {self.auth_header: self.api_key}
        if self.username or self.password:
            token = base64.b64encode(f"{self.username}:{self.password}".encode("utf-8")).decode("ascii")
            return {"Authorization": f"Basic {token}"}
        return {}

    def envelope(self, encoded_events: Iterable[str]) -> bytes:
        return _build_ingest_body(encoded_events, self.sdk_name, self.sdk_version, self.service)

    def send_batch(self, encoded_events: Iterable[str]) -> CollectorResponse:
        body = self.envelope(encoded_events)
        headers = {"content-type": "application/json"}
        headers.update(self._auth_headers())
        request = Request(self.endpoint, data=body, headers=headers, method="POST")
        last_error: Exception | None = None
        for attempt in range(self.retries + 1):
            try:
                try:
                    with urlopen(request, timeout=self.timeout) as response:
                        status = getattr(response, "status", response.getcode())
                        headers_obj = getattr(response, "headers", None)
                        raw = response.read()
                except HTTPError as exc:
                    status = exc.code
                    headers_obj = exc.headers
                    raw = exc.read()
                payload, parsed = _parse_collector_response_body(raw)
                outcome, reason = _collector_response_outcome(status, parsed)
                if outcome == "success":
                    return parsed
                retry_after = _parse_retry_after(headers_obj.get("Retry-After") if headers_obj else None)
                last_error = RuntimeError(
                    f"collector reported {'retryable batch errors' if outcome == 'retryable' else 'batch failure'}: {reason}"
                )
                if outcome == "retryable" and attempt < self.retries:
                    time.sleep(_retry_delay(attempt, 0.05, 1.0, retry_after))
                    continue
                raise last_error
            except (URLError, TimeoutError, json.JSONDecodeError, ValueError) as exc:
                last_error = exc
                if attempt >= self.retries:
                    break
                time.sleep(min(1.0, 0.05 * (2**attempt)))
        raise RuntimeError(f"collector send failed: {last_error}") from last_error

    def _base_url(self) -> str:
        """Extract host:port base URL from the endpoint."""
        parsed = urlparse(self.endpoint)
        return f"{parsed.scheme}://{parsed.netloc}"

    def health(self) -> bool:
        """Check if the collector is healthy."""
        for path in ("/health", "/healthz"):
            req = Request(f"{self._base_url()}{path}", headers=self._auth_headers(), method="GET")
            try:
                with urlopen(req, timeout=self.timeout) as resp:
                    data = json.loads(resp.read())
                    if data.get("status") == "ok":
                        return True
            except (URLError, HTTPError, OSError, json.JSONDecodeError):
                continue
        return False

    def ready(self) -> bool:
        """Check if the collector is ready to accept requests."""
        for path in ("/ready", "/readyz"):
            req = Request(f"{self._base_url()}{path}", headers=self._auth_headers(), method="GET")
            try:
                with urlopen(req, timeout=self.timeout) as resp:
                    data = json.loads(resp.read())
                    if data.get("status") in ("ok", "ready"):
                        return True
            except (URLError, HTTPError, OSError, json.JSONDecodeError):
                continue
        return False

    def version(self) -> dict:
        """Fetch version info from the collector."""
        req = Request(f"{self._base_url()}/version", headers=self._auth_headers(), method="GET")
        with urlopen(req, timeout=self.timeout) as resp:
            return json.loads(resp.read())

    def status(self) -> dict:
        """Fetch operational status from the collector."""
        req = Request(f"{self._base_url()}/status", headers=self._auth_headers(), method="GET")
        with urlopen(req, timeout=self.timeout) as resp:
            return json.loads(resp.read())

    def tail_lines(
        self,
        timeout: float = 30.0,
        service: str = "",
        kind: str = "",
    ):
        """Stream events from the collector's /tail endpoint.

        Yields parsed event dicts as they arrive. Supports optional
        filtering by service and kind via query parameters.

        Args:
            timeout: Socket read timeout in seconds.
            service: Filter events by service name.
            kind: Filter events by kind (event, log, http, etc.).
        """
        url = self.endpoint.rstrip("/") + "/tail"
        params = []
        if service:
            params.append(("service", service))
        if kind:
            params.append(("kind", kind))
        if params:
            url += "?" + urlencode(params)

        request = Request(url, headers={"accept": "application/x-ndjson", **self._auth_headers()})
        with urlopen(request, timeout=timeout) as response:
            for raw in response:
                line = raw.decode("utf-8").strip()
                if line:
                    try:
                        yield json.loads(line)
                    except json.JSONDecodeError:
                        yield line


def _validate_collector_endpoint(endpoint: str) -> None:
    try:
        parsed = urlparse(endpoint)
        parsed.port
    except ValueError as exc:
        raise ValueError("collector endpoint is malformed") from exc
    if parsed.scheme not in {"http", "https"}:
        raise ValueError("collector endpoint must use http or https")
    if parsed.username is not None or parsed.password is not None:
        raise ValueError("collector endpoint must not contain credentials")
    if not parsed.hostname:
        raise ValueError("collector endpoint must include a host")
    if _private_endpoint_allowed():
        return
    host = parsed.hostname
    if host in {"localhost", "127.0.0.1", "::1"}:
        return
    addresses: set[str] = set()
    try:
        addresses = {item[4][0] for item in socket.getaddrinfo(host, parsed.port or _default_port(parsed.scheme))}
    except socket.gaierror:
        return
    for raw in addresses:
        addr = ip_address(raw)
        if addr.is_private or addr.is_loopback or addr.is_link_local or addr.is_multicast or addr.is_reserved:
            raise ValueError("collector endpoint resolves to a non-public address")


def _validate_basic_auth_endpoint(endpoint: str, username: str, password: str) -> None:
    if not username and password:
        raise ValueError("collector basic auth password requires a username")
    if username and not password and not is_public_dsn_username(username):
        raise ValueError(
            "collector basic auth requires a password unless username is an lx_pub_ capability"
        )
    if not username:
        return
    parsed = urlparse(endpoint)
    if parsed.scheme == "http" and (parsed.hostname or "").lower() not in {"localhost", "127.0.0.1", "::1"}:
        raise ValueError("collector basic auth requires HTTPS except for local endpoints")


def _private_endpoint_allowed() -> bool:
    return getenv("LOZA_ALLOW_PRIVATE_COLLECTOR_ENDPOINTS", "").lower() in {"1", "true", "yes"}


def _default_port(scheme: str) -> int:
    return 443 if scheme == "https" else 80


def WrapHTTPClient(client=None):
    return client or InstrumentedHTTPClient()


def NewRoundTripper(base=None):
    return base or InstrumentedHTTPClient()


# ── CollectorClient API extensions ──────────────────────────────────────────


def _collector_request(
    client: CollectorClient,
    method: str,
    path: str,
    body: Any = None,
    params: dict[str, str] | None = None,
) -> dict:
    from urllib.parse import urlencode

    url = client._base_url().rstrip("/") + "/" + path.lstrip("/")
    if params:
        url += "?" + urlencode(params)
    data = json.dumps(body).encode("utf-8") if body is not None else None
    headers = {"content-type": "application/json"} if body else {}
    headers.update(client._auth_headers())
    req = Request(url, data=data, headers=headers, method=method)
    with urlopen(req, timeout=client.timeout) as resp:
        raw = resp.read()
        return json.loads(raw.decode("utf-8")) if raw else {}


def _collector_validate(self, payload):
    return _collector_request(self, "POST", "/validate", payload)


def _collector_ingest(self, encoded_events):
    return self.send_batch(encoded_events)


def _collector_query(self, **params):
    return _collector_request(self, "POST", "/query", body=params)


def _collector_tail(self, **params):
    return _collector_request(self, "GET", "/tail", params=params)


def _collector_delete(self, **params):
    event_id = params.get("event_id") or params.get("id")
    if event_id:
        return _collector_request(self, "DELETE", f"/events/{event_id}")
    tenant_id = params.get("tenant_id")
    if tenant_id:
        return _collector_request(self, "DELETE", f"/events/by-tenant/{tenant_id}")
    user_id = params.get("user_id")
    if user_id:
        return _collector_request(self, "DELETE", f"/events/by-user/{user_id}")
    raise ValueError("delete requires one of: event_id/id, tenant_id, user_id")


def _collector_replay(self, **params):
    return _collector_request(self, "POST", "/replay", body=params)


def _collector_dlq_list(self, **params):
    return _collector_request(self, "GET", "/dlq", params=params)


def _collector_dlq_read(self, dlq_id):
    return _collector_request(self, "GET", f"/dlq/{dlq_id}")


def _collector_dlq_replay(self, dlq_id):
    return _collector_request(self, "POST", f"/dlq/{dlq_id}/replay")


def _collector_keys_create(self, **params):
    return _collector_request(self, "POST", "/keys", body=params)


def _collector_keys_revoke(self, key_id):
    return _collector_request(self, "DELETE", f"/keys/{key_id}")


def _collector_keys_rotate(self, key_id):
    return _collector_request(self, "POST", f"/keys/{key_id}/rotate")


def _collector_sinks_list(self):
    return _collector_request(self, "GET", "/sinks")


def _collector_sinks_test(self, name):
    return _collector_request(self, "POST", f"/sinks/{name}/test")


def _collector_policy_validate(self, policy):
    return _collector_request(self, "POST", "/policy/validate", policy)


def _collector_schema_check(self, event):
    return _collector_request(self, "POST", "/schema/check", event)


def _collector_schema_publish(self, schema):
    return _collector_request(self, "POST", "/schema/publish", schema)


def _collector_retention_apply(self, policy=None):
    return _collector_request(self, "POST", "/retention/apply", body=policy or {})


CollectorClient.validate = _collector_validate
CollectorClient.ingest = _collector_ingest
CollectorClient.query = _collector_query
CollectorClient.tail = _collector_tail
CollectorClient.delete = _collector_delete
CollectorClient.replay = _collector_replay
CollectorClient.dlq_list = _collector_dlq_list
CollectorClient.dlq_read = _collector_dlq_read
CollectorClient.dlq_replay = _collector_dlq_replay
CollectorClient.keys_create = _collector_keys_create
CollectorClient.keys_revoke = _collector_keys_revoke
CollectorClient.keys_rotate = _collector_keys_rotate
CollectorClient.sinks_list = _collector_sinks_list
CollectorClient.sinks_test = _collector_sinks_test
CollectorClient.policy_validate = _collector_policy_validate
CollectorClient.schema_check = _collector_schema_check
CollectorClient.schema_publish = _collector_schema_publish
CollectorClient.retention_apply = _collector_retention_apply
