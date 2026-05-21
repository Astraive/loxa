from __future__ import annotations

from ... import Finish, FinishError, Params, StartHTTPEvent, Emit

class Middleware:
    def __init__(self, app=None, service: str = "") -> None:
        self.app = app
        self.service = service
    def wrap_scope(self, scope: dict) -> object:
        return StartHTTPEvent(None, Params(event="http.request", kind="http", method=scope.get("method", ""), path=scope.get("path", ""), service=self.service))

def middleware(app=None, **config):
    return Middleware(app, **config)
