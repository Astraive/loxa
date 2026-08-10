from __future__ import annotations

from typing import Any

from ... import Info


class StructlogProcessor:
    def __init__(self, emit_immediate: bool = True) -> None:
        self.emit_immediate = emit_immediate

    def __call__(self, logger, method_name: str, event_dict: dict[str, Any]):
        if self.emit_immediate:
            message = str(event_dict.pop("event", method_name))
            Info(message, **event_dict)
        return event_dict


def bind_loza(logger):
    return logger


def processor(**kwargs) -> StructlogProcessor:
    return StructlogProcessor(**kwargs)
