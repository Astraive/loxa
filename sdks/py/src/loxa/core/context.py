from __future__ import annotations

from .event import EventContext

def FromContext(ctx: object) -> tuple[EventContext | None, bool]:
    return (ctx, isinstance(ctx, EventContext))

def HasEvent(ctx: object) -> bool:
    return isinstance(ctx, EventContext)

def EventID(ctx: EventContext) -> str:
    return ctx.event_id

def RequestIDFromContext(ctx: EventContext) -> str:
    return ctx.params.request_id or ctx.request_id
