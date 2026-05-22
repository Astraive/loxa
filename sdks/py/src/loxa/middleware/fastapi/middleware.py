from __future__ import annotations

import time
from typing import Any, Awaitable, Callable

from ... import Bool, Emit, Enrich, Finish, FinishError, Int, Params, StartHTTPEvent, String


class Middleware:
    """FastAPI/ASGI middleware that captures request lifecycle as one LOXA wide event."""

    def __init__(self, app=None, service: str = "", route_resolver=None) -> None:
        self.app = app
        self.service = service
        self.route_resolver = route_resolver

    def wrap_scope(self, scope: dict) -> object:
        route = self.route_resolver(scope) if self.route_resolver else scope.get("path", "")
        return StartHTTPEvent(None, Params(
            event="http.request", kind="http",
            method=scope.get("method", ""),
            path=scope.get("path", ""),
            route=route,
            service=self.service,
        ))

    async def __call__(self, scope: dict, receive: Callable, send: Callable) -> None:
        if self.app is None:
            return
        if scope.get("type") != "http":
            await self.app(scope, receive, send)
            return
        status_code = 500
        response_bytes = 0
        started = time.perf_counter()
        ctx = self.wrap_scope(scope)

        async def capture_send(message):
            nonlocal status_code, response_bytes
            if message.get("type") == "http.response.start":
                status_code = int(message.get("status", 200))
            elif message.get("type") == "http.response.body":
                response_bytes += len(message.get("body", b""))
            await send(message)

        try:
            Enrich(ctx,
                String("http.user_agent", _header(scope, b"user-agent")),
                String("http.remote_ip", _remote_ip(scope)),
            )
            await self.app(scope, receive, capture_send)
            Finish(ctx, "error" if status_code >= 500 else "success",
                Int("status_code", status_code),
                Int("response_bytes", response_bytes),
                Int("duration_ms", int((time.perf_counter() - started) * 1000)),
            )
        except Exception as exc:
            FinishError(ctx, exc, Bool("panic", True))
            raise
        finally:
            Emit(ctx)


def _header(scope, wanted):
    for key, value in scope.get("headers", []):
        if key.lower() == wanted:
            return value.decode("latin1")
    return ""


def _remote_ip(scope):
    client = scope.get("client")
    if isinstance(client, (tuple, list)) and client:
        return str(client[0])
    return ""


def middleware(app=None, **config):
    return Middleware(app, **config)
