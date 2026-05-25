from __future__ import annotations

from .event import Params


def http_params(event: str = "http.request", method: str = "", path: str = "") -> Params:
    return Params(event=event, kind="http", method=method, path=path)


def job_params(name: str) -> Params:
    return Params(event=name, kind="job")
