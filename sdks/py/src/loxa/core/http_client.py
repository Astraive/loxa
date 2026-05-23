from __future__ import annotations

import json
import time
from datetime import datetime, timezone
from email.utils import parsedate_to_datetime
from typing import Iterable
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen
from ..generated.spec_contract import (
    CollectorResponse,
    LOXA_INGEST_API_VERSION,
    build_ingest_envelope,
    parse_collector_response,
    validate_event_payload,
)


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
    return response.error or response.reason or f"accepted={response.accepted} rejected={response.rejected} invalid={response.invalid}"


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
    if payload.get("api_version") != LOXA_INGEST_API_VERSION:
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
        sdk_name: str = "loxa-py",
        sdk_version: str = "0.0.1",
        service: str = "",
    ) -> None:
        self.endpoint = endpoint
        self.api_key = api_key
        self.auth_header = auth_header
        self.timeout = timeout
        self.retries = retries
        self.sdk_name = sdk_name
        self.sdk_version = sdk_version
        self.service = service

    def envelope(self, encoded_events: Iterable[str]) -> bytes:
        return _build_ingest_body(encoded_events, self.sdk_name, self.sdk_version, self.service)

    def send_batch(self, encoded_events: Iterable[str]) -> CollectorResponse:
        body = self.envelope(encoded_events)
        headers = {"content-type": "application/json"}
        if self.api_key:
            if self.auth_header.lower() == "authorization":
                headers[self.auth_header] = f"Bearer {self.api_key}"
            else:
                headers[self.auth_header] = self.api_key
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
        from urllib.parse import urlparse
        parsed = urlparse(self.endpoint)
        return f"{parsed.scheme}://{parsed.netloc}"

    def health(self) -> bool:
        """Check if the collector is healthy."""
        req = Request(f"{self._base_url()}/healthz", method="GET")
        try:
            with urlopen(req, timeout=self.timeout) as resp:
                data = json.loads(resp.read())
                return data.get("status") == "ok"
        except (URLError, HTTPError, OSError, json.JSONDecodeError):
            return False

    def ready(self) -> bool:
        """Check if the collector is ready to accept requests."""
        req = Request(f"{self._base_url()}/readyz", method="GET")
        try:
            with urlopen(req, timeout=self.timeout) as resp:
                data = json.loads(resp.read())
                return data.get("status") in ("ok", "ready")
        except (URLError, HTTPError, OSError, json.JSONDecodeError):
            return False

    def version(self) -> dict:
        """Fetch version info from the collector."""
        req = Request(f"{self._base_url()}/version", method="GET")
        with urlopen(req, timeout=self.timeout) as resp:
            return json.loads(resp.read())

    def status(self) -> dict:
        """Fetch operational status from the collector."""
        req = Request(f"{self._base_url()}/v1/status", method="GET")
        if self.api_key:
            if self.auth_header.lower() == "authorization":
                req.add_header(self.auth_header, f"Bearer {self.api_key}")
            else:
                req.add_header(self.auth_header, self.api_key)
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
            params.append(f"service={service}")
        if kind:
            params.append(f"kind={kind}")
        if params:
            url += "?" + "&".join(params)

        request = Request(url, headers={"accept": "application/x-ndjson"})
        with urlopen(request, timeout=timeout) as response:
            for raw in response:
                line = raw.decode("utf-8").strip()
                if line:
                    try:
                        yield json.loads(line)
                    except json.JSONDecodeError:
                        yield line


def WrapHTTPClient(client=None):
    return client or InstrumentedHTTPClient()


def NewRoundTripper(base=None):
    return base or InstrumentedHTTPClient()


# ── CollectorClient API extensions ──────────────────────────────────────────

def _collector_request(
    client: CollectorClient, method: str, path: str, body: Any = None,
    params: dict[str, str] | None = None,
) -> dict:
    from urllib.parse import urlencode

    url = client._base_url().rstrip("/") + "/" + path.lstrip("/")
    if params:
        url += "?" + urlencode(params)
    data = json.dumps(body).encode("utf-8") if body is not None else None
    headers = {"content-type": "application/json"} if body else {}
    if client.api_key:
        if client.auth_header.lower() == "authorization":
            headers[client.auth_header] = f"Bearer {client.api_key}"
        else:
            headers[client.auth_header] = client.api_key
    req = Request(url, data=data, headers=headers, method=method)
    with urlopen(req, timeout=client.timeout) as resp:
        raw = resp.read()
        return json.loads(raw.decode("utf-8")) if raw else {}


def _collector_validate(self, payload): return _collector_request(self, "POST", "/v1/validate", payload)
def _collector_ingest(self, encoded_events): return self.send_batch(encoded_events)
def _collector_query(self, **params): return _collector_request(self, "GET", "/v1/query", params=params)
def _collector_tail(self, **params): return _collector_request(self, "GET", "/v1/tail", params=params)
def _collector_delete(self, **params): return _collector_request(self, "DELETE", "/v1/events", params=params)
def _collector_replay(self, **params): return _collector_request(self, "POST", "/v1/replay", params=params)
def _collector_dlq_list(self, **params): return _collector_request(self, "GET", "/v1/dlq", params=params)
def _collector_dlq_read(self, dlq_id): return _collector_request(self, "GET", f"/v1/dlq/{dlq_id}")
def _collector_dlq_replay(self, dlq_id): return _collector_request(self, "POST", f"/v1/dlq/{dlq_id}/replay")
def _collector_keys_create(self, **params): return _collector_request(self, "POST", "/v1/keys", params=params)
def _collector_keys_revoke(self, key_id): return _collector_request(self, "DELETE", f"/v1/keys/{key_id}")
def _collector_sinks_list(self): return _collector_request(self, "GET", "/v1/sinks")
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
CollectorClient.sinks_list = _collector_sinks_list
