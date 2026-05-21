from __future__ import annotations

from datetime import datetime, timezone

class Clock:
    def now(self) -> datetime:
        return datetime.now(timezone.utc)

class FrozenClock(Clock):
    def __init__(self, value: datetime) -> None:
        self.value = value
    def now(self) -> datetime:
        return self.value
