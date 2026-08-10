from __future__ import annotations

import json
import threading
from http.server import BaseHTTPRequestHandler, HTTPServer

import loza


class _CollectorHandler(BaseHTTPRequestHandler):
    def _write(self, status: int, payload: dict):
        body = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):  # noqa: N802
        if self.path.startswith("/health"):
            self._write(200, {"status": "ok"})
            return
        if self.path.startswith("/ready"):
            self._write(200, {"status": "ready"})
            return
        if self.path.startswith("/version"):
            self._write(200, {"version": "0.2.0"})
            return
        if self.path.startswith("/status"):
            self._write(200, {"status": "ok"})
            return
        if self.path.startswith("/tail"):
            self._write(200, {"events": []})
            return
        if self.path.startswith("/dlq"):
            self._write(200, {"events": []})
            return
        if self.path.startswith("/sinks"):
            self._write(200, {"sinks": []})
            return
        self._write(404, {"error": "not_found"})

    def do_POST(self):  # noqa: N802
        length = int(self.headers.get("Content-Length", "0") or "0")
        if length > 0:
            _ = self.rfile.read(length)
        if self.path.startswith("/events"):
            self._write(200, {"accepted": 1, "rejected": 0, "invalid": 0})
            return
        if self.path.startswith("/validate"):
            self._write(200, {"valid": True})
            return
        if self.path.startswith("/query"):
            self._write(200, {"rows": []})
            return
        if self.path.startswith("/replay"):
            self._write(202, {"replayed": 1})
            return
        if self.path.startswith("/keys"):
            self._write(200, {"ok": True})
            return
        if self.path.startswith("/policy/validate"):
            self._write(200, {"valid": True, "errors": []})
            return
        if self.path.startswith("/schema/check"):
            self._write(200, {"valid": True})
            return
        if self.path.startswith("/schema/publish"):
            self._write(200, {"published": True})
            return
        if self.path.startswith("/retention/apply"):
            self._write(200, {"applied": True})
            return
        if self.path.startswith("/sinks/") and self.path.endswith("/test"):
            self._write(200, {"status": "healthy"})
            return
        if self.path.startswith("/dlq/") and self.path.endswith("/replay"):
            self._write(200, {"replayed": 1})
            return
        self._write(404, {"error": "not_found"})

    def do_DELETE(self):  # noqa: N802
        if self.path.startswith("/events/") or self.path.startswith("/keys/") or self.path.startswith("/dlq/"):
            self._write(200, {"deleted": 1, "revoked": True})
            return
        self._write(404, {"error": "not_found"})

    def log_message(self, format, *args):  # noqa: A003
        return


def _serve(handler):
    server = HTTPServer(("127.0.0.1", 0), handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    return server


def test_collector_client_family():
    server = _serve(_CollectorHandler)
    try:
        base = f"http://127.0.0.1:{server.server_address[1]}"
        cc = loza.CollectorClient(base + "/events")
        valid_event = {
            "schema_version": "v1",
            "event_version": "v1",
            "event_id": "evt_test_1",
            "timestamp": "2026-05-24T00:00:00Z",
            "service": "verification",
            "event": "verification.event",
            "kind": "event",
            "level": "info",
            "outcome": "success",
            "event_state": "finished",
        }
        assert cc.health() is True
        assert cc.ready() is True
        assert cc.version()["version"] == "0.2.0"
        assert cc.status()["status"] == "ok"
        assert cc.validate({"event": "verification"})["valid"] is True
        assert cc.ingest([json.dumps(valid_event)]).accepted >= 1
        assert isinstance(cc.query(query="select 1"), dict)
        assert isinstance(cc.tail(limit="1"), dict)
        assert isinstance(cc.delete(event_id="evt_1"), dict)
        assert isinstance(cc.replay(event_ids=["evt_1"]), dict)
        assert isinstance(cc.dlq_list(limit="1"), dict)
        assert isinstance(cc.dlq_read("dlq_1"), dict)
        assert isinstance(cc.dlq_replay("dlq_1"), dict)
        assert isinstance(cc.keys_create(name="key"), dict)
        assert isinstance(cc.keys_revoke("key_1"), dict)
        assert isinstance(cc.keys_rotate("key_1"), dict)
        assert isinstance(cc.sinks_list(), dict)
        assert isinstance(cc.sinks_test("stdout"), dict)
        assert cc.policy_validate({"sample_rate": 1.0})["valid"] is True
        assert cc.schema_check({"event": "verification"})["valid"] is True
        assert cc.schema_publish({"schema": "v1"})["published"] is True
        assert cc.retention_apply({"days": 1})["applied"] is True
    finally:
        server.shutdown()
        server.server_close()


def test_cortex_client_family():
    cortex = loza.CortexClient("http://localhost:9312")
    assert hasattr(cortex, "health")
    assert hasattr(cortex, "ready")
    assert hasattr(cortex, "reconstruct")
    assert hasattr(cortex, "service_graph")
    assert hasattr(cortex, "incident_graph")
