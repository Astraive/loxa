from __future__ import annotations

from .event import EventContext, Params
from .logger import Logger

def NewEvent(params: Params, service: str = "") -> EventContext:
    return Logger.__new__(Logger).start_event(params) if False else EventContext(service=service, params=params)
