from __future__ import annotations

import json
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

import pytest

from loza.sinks.httpbatch import HTTPBatchSink


class _Handler(BaseHTTPRequestHandler):
    response_status = 202
    response_body = b'{"status":"accepted","accepted":1}'

    def do_POST(self) -> None:  # noqa: N802
        length = int(self.headers.get("Content-Length", "0"))
        _ = self.rfile.read(length)
        self.send_response(self.response_status)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(self.response_body)

    def log_message(self, fmt: str, *args) -> None:  # noqa: A003
        return None


def _serve(status: int, payload: dict) -> tuple[ThreadingHTTPServer, str]:
    class Handler(_Handler):
        response_status = status
        response_body = json.dumps(payload).encode("utf-8")

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    return server, f"http://127.0.0.1:{server.server_address[1]}"


def test_httpbatch_sink_accepts_clean_batch() -> None:
    server, url = _serve(202, {"status": "accepted", "accepted": 1})
    try:
        sink = HTTPBatchSink(url, service="checkout")
        sink.write(
            '{"schema_version":"v1","event_version":"v1","timestamp":"2026-05-12T00:00:00Z","event_id":"evt_1","service":"checkout","event":"payment.completed","kind":"http"}'
        )
    finally:
        server.shutdown()
        server.server_close()


def test_httpbatch_sink_fails_on_partial_invalid_batch() -> None:
    server, url = _serve(
        207,
        {
            "status": "partial",
            "accepted": 1,
            "invalid": 1,
            "acks": [
                {"event_id": "evt_1", "status": "accepted", "reason": "accepted"},
                {
                    "event_id": "evt_2",
                    "status": "invalid",
                    "reason": "schema_invalid",
                    "message": "schema validation failed",
                },
            ],
        },
    )
    try:
        sink = HTTPBatchSink(url, service="checkout")
        with pytest.raises(RuntimeError, match="schema_invalid"):
            sink.write_batch(
                [
                    '{"schema_version":"v1","event_version":"v1","timestamp":"2026-05-12T00:00:00Z","event_id":"evt_1","service":"checkout","event":"payment.completed","kind":"http"}',
                    '{"schema_version":"v1","event_version":"v1","timestamp":"2026-05-12T00:00:01Z","event_id":"evt_2","service":"checkout","event":"payment.completed","kind":"http"}',
                ]
            )
    finally:
        server.shutdown()
        server.server_close()
