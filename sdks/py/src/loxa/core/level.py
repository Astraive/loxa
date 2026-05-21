from __future__ import annotations

LevelDebug = "debug"
LevelInfo = "info"
LevelWarn = "warn"
LevelError = "error"
LevelFatal = "fatal"

def ParseLevel(value: str) -> str:
    lowered = (value or "").strip().lower()
    return lowered if lowered in {LevelDebug, LevelInfo, LevelWarn, LevelError, LevelFatal} else LevelInfo
