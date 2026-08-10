from __future__ import annotations

import asyncio
import json

import loza
from loza.middleware.asgi import Middleware


async def _receive() -> dict:
    return {"type": "http.request", "body": b"", "more_body": False}


async def _run_app(messages: list[dict]) -> None:
    async def app(scope, receive, send):
        await send({"type": "http.response.start", "status": 201, "headers": []})
        await send({"type": "http.response.body", "body": b"ok"})

    sink = loza.MemorySink()
    loza.Configure(loza.Test("checkout").with_sink(sink))
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


def test_flask_middleware_streams_response_without_buffering() -> None:
    from loza.middleware.flask import Middleware as FlaskMiddleware

    produced = 0

    def app(_environ, start_response):
        start_response("200 OK", [])

        def body():
            nonlocal produced
            for chunk in (b"one", "two"):
                produced += 1
                yield chunk

        return body()

    sink = loza.MemorySink()
    loza.Configure(loza.Test("checkout").with_sink(sink))
    middleware = FlaskMiddleware(app, service="checkout")

    response = middleware({"REQUEST_METHOD": "GET", "PATH_INFO": "/stream"}, lambda *_args: None)

    assert produced == 0
    iterator = iter(response)
    assert next(iterator) == b"one"
    assert produced == 1
    assert next(iterator) == b"two"
    assert produced == 2

    try:
        next(iterator)
    except StopIteration:
        pass
    else:  # pragma: no cover
        raise AssertionError("response should be exhausted")

    assert len(sink.events) == 1
    payload = json.loads(sink.events[0])
    assert payload["attrs"]["response_bytes"] == 6


def test_flask_middleware_closes_original_iterable_on_close() -> None:
    from loza.middleware.flask import Middleware as FlaskMiddleware

    class StreamingBody:
        def __init__(self) -> None:
            self.closed = False

        def __iter__(self):
            yield b"first"
            yield b"second"

        def close(self) -> None:
            self.closed = True

    body = StreamingBody()

    def app(_environ, start_response):
        start_response("200 OK", [])
        return body

    sink = loza.MemorySink()
    loza.Configure(loza.Test("checkout").with_sink(sink))
    middleware = FlaskMiddleware(app, service="checkout")

    response = middleware({"REQUEST_METHOD": "GET", "PATH_INFO": "/stream"}, lambda *_args: None)
    iterator = iter(response)

    assert next(iterator) == b"first"
    iterator.close()

    assert body.closed is True
    assert len(sink.events) == 1
    payload = json.loads(sink.events[0])
    assert payload["attrs"]["response_bytes"] == 5
