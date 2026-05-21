from __future__ import annotations

from typing import Any

from ... import Error, Info, Warn


class LoguruSink:
    def __call__(self, message: Any) -> None:
        record = getattr(message, "record", {})
        text = str(getattr(message, "message", message)).strip()
        level = str(record.get("level", {}).get("name", "INFO")).lower() if isinstance(record, dict) else "info"
        if level in {"error", "critical"}:
            Error(text)
        elif level == "warning":
            Warn(text)
        else:
            Info(text)


def bind_loxa(logger):
    return logger


def sink() -> LoguruSink:
    return LoguruSink()
