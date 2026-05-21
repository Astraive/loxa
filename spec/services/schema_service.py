#!/usr/bin/env python3
"""Simple schema-service prototype.

Serves generated contract and JSON Schema files with ETag/Last-Modified caching, a /health endpoint,
and a lightweight /metrics endpoint. Intended as a local prototype for a durable schema/contract
registry (cacheable, with health checks) that SDKs and CI can query.

Run:
  python loxa-spec\services\schema_service.py --host 0.0.0.0 --port 8080

This intentionally has no external dependencies and is easy to run in CI or as a lightweight
container during rollout. It should be replaced by a production service (API gateway + CDN)
for high-scale deployments.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import logging
import mimetypes
import os
import threading
import time
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Optional

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")


def _env_int(name: str, default: int) -> int:
    value = os.getenv(name)
    if value is None:
        return default
    try:
        return int(value)
    except ValueError:
        return default


class SchemaServiceHandler(BaseHTTPRequestHandler):
    server_version = "LoxaSchemaService/0.1"

    def _spec_root(self) -> Path:
        # service located in loxa-spec/services/ -> step up one to loxa-spec
        return Path(__file__).resolve().parents[1]

    def _send_json(self, obj: object, status: int = HTTPStatus.OK) -> None:
        data = json.dumps(obj).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def _send_text(self, text: str, status: int = HTTPStatus.OK, content_type: str = "text/plain; charset=utf-8"):
        data = text.encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def _file_etag(self, path: Path) -> str:
        try:
            stat = path.stat()
            # Use mtime and size for a stable ETag; fallback to sha256 for extra safety
            return f'W/"{int(stat.st_mtime)}-{stat.st_size}"'
        except Exception:
            # conservative fallback: hash contents
            try:
                h = hashlib.sha256()
                with path.open("rb") as f:
                    while True:
                        chunk = f.read(8192)
                        if not chunk:
                            break
                        h.update(chunk)
                return f'"{h.hexdigest()}"'
            except Exception:
                return '"unknown"'

    def _send_file(self, path: Path) -> None:
        if not path.exists():
            self.send_error(HTTPStatus.NOT_FOUND, "not found")
            return
        etag = self._file_etag(path)
        last_mod = time.gmtime(path.stat().st_mtime)
        last_mod_str = time.strftime("%a, %d %b %Y %H:%M:%S GMT", last_mod)

        if_none_match = self.headers.get("If-None-Match")
        if_modified_since = self.headers.get("If-Modified-Since")

        if if_none_match and if_none_match == etag:
            self.send_response(HTTPStatus.NOT_MODIFIED)
            self.end_headers()
            return
        if if_modified_since and if_modified_since == last_mod_str:
            self.send_response(HTTPStatus.NOT_MODIFIED)
            self.end_headers()
            return

        ctype, _ = mimetypes.guess_type(str(path))
        if not ctype:
            ctype = "application/octet-stream"
        try:
            data = path.read_bytes()
        except Exception:
            self.send_error(HTTPStatus.INTERNAL_SERVER_ERROR, "failed to read file")
            return

        self.send_response(HTTPStatus.OK)
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(data)))
        self.send_header("ETag", etag)
        self.send_header("Last-Modified", last_mod_str)
        # Encourage caching but allow revalidation
        self.send_header("Cache-Control", "public, max-age=60, must-revalidate")
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self) -> None:
        path = self.path
        root = self._spec_root()

        # simple request counting
        self.server.requests_count = getattr(self.server, "requests_count", 0) + 1

        if path == "/health":
            self._send_json({"status": "ok"})
            return

        if path == "/metrics":
            # Basic metrics: uptime, requests served, contract hits
            uptime = time.time() - self.server.start_time
            requests = getattr(self.server, "requests_count", 0)
            contract_hits = getattr(self.server, "contract_hits", 0)
            metrics = (
                "# HELP loxa_schema_service_uptime_seconds Uptime in seconds\n"
                "# TYPE loxa_schema_service_uptime_seconds gauge\n"
                f"loxa_schema_service_uptime_seconds {uptime:.2f}\n"
                "# HELP loxa_schema_service_requests_total Total requests served\n"
                "# TYPE loxa_schema_service_requests_total counter\n"
                f"loxa_schema_service_requests_total {requests}\n"
                "# HELP loxa_schema_service_contract_requests Total contract requests\n"
                "# TYPE loxa_schema_service_contract_requests counter\n"
                f"loxa_schema_service_contract_requests {contract_hits}\n"
            )
            self._send_text(metrics)
            return

        if path == "/contract" or path == "/contract/loxa-contract.json":
            target = root / "generated" / "contract" / "loxa-contract.json"
            # track contract fetches
            self.server.contract_hits = getattr(self.server, "contract_hits", 0) + 1
            return self._send_file(target)

        if path.startswith("/schemas/"):
            rel = path[len("/schemas/"):]
            # Base dir: spec/schemas/json/
            target = root / "spec" / "schemas" / "json" / rel
            # protect against path traversal
            try:
                target = target.resolve()
                if not str(target).startswith(str((root / "spec" / "schemas" / "json").resolve())):
                    self.send_error(HTTPStatus.FORBIDDEN, "forbidden")
                    return
            except Exception:
                self.send_error(HTTPStatus.NOT_FOUND, "not found")
                return
            return self._send_file(target)

        # default: simple index with links
        if path == "/" or path == "/index.html":
            index = {
                "service": "loxa-spec schema-service",
                "endpoints": ["/health", "/contract", "/schemas/<name>", "/metrics"],
            }
            self._send_json(index)
            return

        self.send_error(HTTPStatus.NOT_FOUND, "not found")


def run_server(host: str = "127.0.0.1", port: int = 8080) -> None:
    server = ThreadingHTTPServer((host, port), SchemaServiceHandler)
    server.start_time = time.time()
    logging.info("Starting schema-service on %s:%d", host, port)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        logging.info("Shutting down schema-service")
        server.shutdown()


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--host", default=os.getenv("SCHEMA_SERVICE_HOST", "127.0.0.1"))
    parser.add_argument("--port", type=int, default=_env_int("SCHEMA_SERVICE_PORT", 8080))
    args = parser.parse_args()
    run_server(args.host, args.port)
