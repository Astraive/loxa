from __future__ import annotations

import time

from ... import Bool, Emit, Enrich, Finish, FinishError, Int, Params, StartHTTPEvent, String


class LoxaMiddleware:
    """Django middleware that captures request lifecycle as one LOXA wide event."""

    def __init__(self, get_response=None, service: str = "") -> None:
        self.get_response = get_response
        self.service = service

    def __call__(self, request):
        started = time.perf_counter()
        ctx = StartHTTPEvent(None, Params(
            event="http.request", kind="http",
            method=request.method or "",
            path=request.path,
            route=request.path,
            service=self.service,
        ))
        try:
            Enrich(ctx,
                String("http.user_agent", request.META.get("HTTP_USER_AGENT", "")),
                String("http.remote_ip", request.META.get("REMOTE_ADDR", "")),
            )
            response = self.get_response(request)
            Finish(ctx, "error" if response.status_code >= 500 else "success",
                Int("status_code", response.status_code),
                Int("duration_ms", int((time.perf_counter() - started) * 1000)),
            )
            return response
        except Exception as exc:
            FinishError(ctx, exc, Bool("panic", True))
            raise
        finally:
            Emit(ctx)


def middleware(get_response=None, **config):
    return LoxaMiddleware(get_response, **config)
