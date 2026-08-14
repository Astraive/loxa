from __future__ import annotations

import base64
import gzip as gzip_mod
import time
import urllib.request
from dataclasses import dataclass, field
from typing import Any, Iterable
from urllib.error import HTTPError, URLError

# TimeoutError was added in Python 3.11; define fallback for older versions
try:
    TimeoutError
except NameError:
    TimeoutError = OSError  # type: ignore[assignment,misc]

from ...core.http_client import (
    _build_ingest_body,
    _collector_response_outcome,
    _parse_collector_response_body,
    _parse_retry_after,
    _retry_delay,
    _validate_basic_auth_endpoint,
    _validate_collector_endpoint,
)
from ...version import SDK_VERSION


@dataclass(slots=True)
class HTTPBatchSink:
    """Collector HTTP batch sink.

    The SDK stays lightweight: this sink only serializes a batch envelope and
    posts it to a collector endpoint. Retries, routing, fanout, and heavy
    backend delivery remain collector responsibilities.
    """

    endpoint: str
    api_key: str = ""
    username: str = ""
    password: str = ""
    auth_header: str = "Authorization"
    sdk_name: str = "loza-py"
    sdk_version: str = SDK_VERSION
    service: str = ""
    timeout: float = 2.0
    retries: int = 2
    base_delay: float = 0.1
    max_delay: float = 30.0
    enable_compression: bool = True
    ndjson: bool = False
    stats_handler: Any = None
    _last_response: dict | None = field(default=None, init=False, repr=False)
    def __post_init__(self) -> None:
        _validate_collector_endpoint(self.endpoint)
        _validate_basic_auth_endpoint(self.endpoint, self.username, self.password)

    def _auth_headers(self) -> dict[str, str]:
        if self.api_key:
            if self.auth_header.lower() == "authorization":
                return {self.auth_header: f"Bearer {self.api_key}"}
            return {self.auth_header: self.api_key}
        if self.username or self.password:
            token = base64.b64encode(f"{self.username}:{self.password}".encode("utf-8")).decode("ascii")
            return {"Authorization": f"Basic {token}"}
        return {}

    def write(self, encoded: str) -> None:
        self.write_batch([encoded])

    def write_batch(self, encoded_events: Iterable[str]) -> None:
        events_list = list(encoded_events)
        headers: dict[str, str] = {}

        if self.ndjson:
            payload = "\n".join(e.rstrip() for e in events_list).encode("utf-8")
            headers["content-type"] = "application/x-ndjson"
        else:
            payload = _build_ingest_body(events_list, self.sdk_name, self.sdk_version, self.service)
            headers["content-type"] = "application/json"

        if self.enable_compression:
            payload = gzip_mod.compress(payload)
            headers["content-encoding"] = "gzip"

        headers.update(self._auth_headers())

        req = urllib.request.Request(self.endpoint, data=payload, headers=headers, method="POST")
        last_error: Exception | None = None
        for attempt in range(self.retries + 1):
            try:
                try:
                    with urllib.request.urlopen(req, timeout=self.timeout) as response:
                        status = getattr(response, "status", response.getcode())
                        headers_obj = getattr(response, "headers", None)
                        raw = response.read()
                except HTTPError as exc:
                    status = exc.code
                    headers_obj = exc.headers
                    raw = exc.read()
                payload_dict, parsed = _parse_collector_response_body(raw)
                self._last_response = payload_dict or None
                self._notify_collector_ack(parsed)
                outcome, reason = _collector_response_outcome(status, parsed)
                if outcome == "success":
                    return
                retry_after = _parse_retry_after(headers_obj.get("Retry-After") if headers_obj else None)
                last_error = RuntimeError(
                    f"collector reported {'retryable batch errors' if outcome == 'retryable' else 'batch failure'}: {reason}"
                )
                if outcome == "retryable" and attempt < self.retries:
                    time.sleep(_retry_delay(attempt, self.base_delay, self.max_delay, retry_after))
                    continue
                raise last_error
            except (URLError, TimeoutError) as exc:
                last_error = exc
                if attempt < self.retries:
                    time.sleep(_retry_delay(attempt, self.base_delay, self.max_delay))
                    continue
                raise RuntimeError(f"collector send failed: {exc}") from exc
        if last_error is not None:
            raise RuntimeError(f"collector send failed: {last_error}") from last_error

    def _notify_collector_ack(self, parsed: Any) -> None:
        if self.stats_handler is None or parsed is None:
            return
        callback = getattr(self.stats_handler, "on_collector_ack", None)
        if callback is None:
            return
        try:
            callback(
                acks=getattr(parsed, "acks", []),
                errors=getattr(parsed, "errors", []),
                request_id=getattr(parsed, "request_id", ""),
                deduped=getattr(parsed, "deduped", 0),
            )
        except Exception:
            pass

    @property
    def last_collector_response(self) -> dict | None:
        return self._last_response

    def flush(self) -> None:
        return None

    def close(self) -> None:
        return None


class NoopSink:
    def write(self, encoded: str) -> None:
        return None

    def flush(self) -> None:
        return None

    def close(self) -> None:
        return None


__all__ = ["HTTPBatchSink", "NoopSink"]
