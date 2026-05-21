from __future__ import annotations

import asyncio
import json

import loxa
from loxa.middleware.asgi import Middleware


async def _receive() -> dict:
    return {"type": "http.request", "body": b"", "more_body": False}


async def _run_app(messages: list[dict]) -> None:
    async def app(scope, receive, send):
        await send({"type": "http.response.start", "status": 201, "headers": []})
        await send({"type": "http.response.body", "body": b"ok"})

    sink = loxa.MemorySink()
    loxa.Configure(loxa.Test("checkout").with_sink(sink))
    middleware = Middleware(app, service="checkout")

    async def send(message: dict) -> None:
        messages.append(message)

    await middleware(
        {
            "type": "http",
            "method": "GET",
            "path": "/health",
            "headers": [(b"user-agent", b"pytest")],
            "client": ("127.0.0.1", 1234),
        },
        _receive,
        send,
    )

    assert len(sink.events) == 1
    payload = json.loads(sink.events[0])
    assert payload["kind"] == "http"
    assert payload["http"]["user_agent"] == "pytest"
    assert payload["http"]["remote_ip"] == "127.0.0.1"
    # status_code is a canonical field — canonical enforcement removes it from attrs
    assert "status_code" not in payload.get("attrs", {})
    assert payload["attrs"]["response_bytes"] == 2


def test_asgi_middleware_captures_http_request() -> None:
    asyncio.run(_run_app([]))
