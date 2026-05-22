from __future__ import annotations

import time
from typing import Any, Callable, Iterable

from ... import Bool, Emit, Enrich, Finish, FinishError, Int, Params, StartHTTPEvent, String


class Middleware:
    """Flask/WSGI middleware that captures request lifecycle as one LOXA wide event."""

    def __init__(self, app=None, service: str = "") -> None:
        self.app = app
        self.service = service

    def wrap_scope(self, environ: dict) -> object:
        return StartHTTPEvent(None, Params(
            event="http.request", kind="http",
            method=environ.get("REQUEST_METHOD", ""),
            path=environ.get("PATH_INFO", ""),
            route=environ.get("PATH_INFO", ""),
            service=self.service,
        ))

    def __call__(self, environ: dict, start_response: Callable) -> Iterable[bytes]:
        if self.app is None:
            return []
        status_code = 500
        response_bytes = 0
        started = time.perf_counter()
        ctx = self.wrap_scope(environ)

        def capture_start_response(status, response_headers, exc_info=None):
            nonlocal status_code
            status_code = int(status.split()[0]) if status else 500
            return start_response(status, response_headers, exc_info)

        try:
            Enrich(ctx,
                String("http.user_agent", environ.get("HTTP_USER_AGENT", "")),
                String("http.remote_ip", environ.get("REMOTE_ADDR", "")),
            )
            response = self.app(environ, capture_start_response)
            chunks = []
            for chunk in response:
                if isinstance(chunk, bytes):
                    response_bytes += len(chunk)
                else:
                    chunk = chunk.encode("utf-8")
                    response_bytes += len(chunk)
                chunks.append(chunk)
            Finish(ctx, "error" if status_code >= 500 else "success",
                Int("status_code", status_code),
                Int("response_bytes", response_bytes),
                Int("duration_ms", int((time.perf_counter() - started) * 1000)),
            )
            return chunks
        except Exception as exc:
            FinishError(ctx, exc, Bool("panic", True))
            raise
        finally:
            Emit(ctx)


def middleware(app=None, **config):
    return Middleware(app, **config)
