from __future__ import annotations

from dataclasses import replace

from .event import Params


def http_params(method: str, path: str, *, service: str = "", route: str = "", name: str = "") -> Params:
    return Params(
        service=service,
        name=name or f"{method} {path}",
        event=name or f"{method} {path}",
        kind="http",
        method=method,
        path=path,
        route=route,
    )


def job_params(name: str, *, service: str = "") -> Params:
    return Params(service=service, name=name, event=name, kind="job")


def queue_params(queue: str, message_id: str = "", *, service: str = "") -> Params:
    return Params(service=service, name=queue, event=queue, kind="queue", request_id=message_id)


def cli_params(command: str, *, service: str = "") -> Params:
    return Params(service=service, name=command, event=command, kind="cli")


def cron_params(job: str, *, service: str = "") -> Params:
    return Params(service=service, name=job, event=job, kind="cron")


def with_trace(params: Params, trace_id: str, span_id: str = "") -> Params:
    return replace(params, trace_id=trace_id, span_id=span_id)


__all__ = [
    "Params",
    "http_params",
    "job_params",
    "queue_params",
    "cli_params",
    "cron_params",
    "with_trace",
]
