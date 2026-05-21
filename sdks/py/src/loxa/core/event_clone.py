from __future__ import annotations

import copy
from .event import EventContext

def clone_event(event: EventContext) -> EventContext:
    return copy.deepcopy(event)
