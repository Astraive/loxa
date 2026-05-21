from __future__ import annotations

import json
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

import pytest

from loxa.sinks.httpbatch import HTTPBatchSink


def _repo_root() -> Path:
    return Path(__file__).resolve().parents[2]


def _fixtures() -> list[Path]:
    root = _repo_root() / "loxa-spec" / "examples" / "golden" / "collector-acks"
    return sorted(root.glob("*.json"))


def _serve(status: int, body: bytes) -> tuple[ThreadingHTTPServer, str]:
    class Handler(BaseHTTPRequestHandler):
        def do_POST(self) -> None:  # noqa: N802
            length = int(self.headers.get("Content-Length", "0"))
            _ = self.rfile.read(length)
            self.send_response(status)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, fmt: str, *args) -> None:  # noqa: A003
            return None

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    return server, f"http://127.0.0.1:{server.server_address[1]}"


def test_collector_ack_behavior_fixtures() -> None:
    for path in _fixtures():
        fixture = json.loads(path.read_text())
        server, url = _serve(fixture["http_status"], json.dumps(fixture["response"]).encode("utf-8"))
        try:
            sink = HTTPBatchSink(url, service="checkout")
            if fixture["expected"]["outcome"] == "success":
                sink.write('{"event_id":"evt_1","service":"checkout","event":"payment.completed","schema_version":"v1","event_version":"v1","timestamp":"2026-01-01T00:00:00Z"}')
            else:
                with pytest.raises(RuntimeError, match=fixture["expected"].get("message_contains", "")):
                    sink.write('{"event_id":"evt_1","service":"checkout","event":"payment.completed","schema_version":"v1","event_version":"v1","timestamp":"2026-01-01T00:00:00Z"}')
        finally:
            server.shutdown()
            server.server_close()
