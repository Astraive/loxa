from __future__ import annotations

import json
import threading
from http.server import BaseHTTPRequestHandler, HTTPServer

from lql_client import Client, ConnectionConfig, ErrorCategory, QueryValue


class Handler(BaseHTTPRequestHandler):
    body = None
    headers = None

    def do_POST(self):
        Handler.headers = self.headers
        Handler.body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        payload = {"columns": [{"name": "event_id", "type": "string"}], "rows": [{"event_id": "evt-1"}], "duration_ms": 3, "row_count": 1}
        encoded = json.dumps(payload).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(encoded)))
        self.end_headers()
        self.wfile.write(encoded)

    def log_message(self, *_args):
        pass


def test_query_uses_scoped_route_and_typed_parameters():
    server = HTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        client = Client(ConnectionConfig(endpoint=f"http://{server.server_address[0]}:{server.server_address[1]}", collector="demo", api_key="api", env="prod", service="cli"))
        result = client.query("from events | where event_id = $id", {"id": QueryValue("evt-1", "string")}, 10)
        assert result.row_count == 1
        assert Handler.headers["Authorization"] == "Bearer api"
        assert Handler.headers["X-Loza-Env"] == "prod"
        assert Handler.headers["X-Loza-Service"] == "cli"
        assert Handler.body["query"].endswith("$id")
        assert Handler.body["parameters"]["id"] == {"type": "string", "value": "evt-1"}
    finally:
        server.shutdown()
        thread.join()


def test_invalid_configuration_has_stable_category():
    try:
        Client(ConnectionConfig(endpoint="http://remote.example", collector="", username="u", password="p"))
    except Exception as error:
        assert getattr(error, "category", None) == ErrorCategory.INVALID_CONFIGURATION
    else:
        raise AssertionError("expected invalid configuration")
