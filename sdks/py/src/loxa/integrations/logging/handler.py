from __future__ import annotations

import logging
from typing import Any

from ... import Error, Info, Warn


class LoxaHandler(logging.Handler):
    """Bridge Python logging records into LOXA immediate events."""

    def __init__(self, level: int = logging.NOTSET, include_extra: bool = True) -> None:
        super().__init__(level)
        self.include_extra = include_extra

    def emit(self, record: logging.LogRecord) -> None:
        attrs = self._attrs(record) if self.include_extra else {}
        if record.exc_info:
            attrs["error.type"] = record.exc_info[0].__name__ if record.exc_info[0] else ""
            attrs["error.message"] = str(record.exc_info[1])
        if record.levelno >= logging.ERROR:
            Error(record.getMessage(), **attrs)
        elif record.levelno >= logging.WARNING:
            Warn(record.getMessage(), **attrs)
        else:
            Info(record.getMessage(), **attrs)

    def _attrs(self, record: logging.LogRecord) -> dict[str, Any]:
        reserved = set(logging.makeLogRecord({}).__dict__)
        attrs = {
            f"log.{key}": value
            for key, value in record.__dict__.items()
            if key not in reserved and _json_safe(value)
        }
        attrs["log.logger"] = record.name
        attrs["log.module"] = record.module
        attrs["log.function"] = record.funcName
        attrs["log.line"] = record.lineno
        return attrs


def _json_safe(value: Any) -> bool:
    return isinstance(value, (str, int, float, bool, type(None), list, tuple, dict))


def Handler(**kwargs) -> LoxaHandler:
    return LoxaHandler(**kwargs)
