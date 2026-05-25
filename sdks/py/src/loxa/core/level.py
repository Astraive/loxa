from __future__ import annotations

LevelDebug = "debug"
LevelInfo = "info"
LevelNotice = "notice"
LevelWarn = "warn"
LevelError = "error"
LevelFatal = "fatal"


def ParseLevel(value: str) -> str:
    lowered = (value or "").strip().lower()
    return lowered if lowered in {LevelDebug, LevelInfo, LevelNotice, LevelWarn, LevelError, LevelFatal} else LevelInfo
