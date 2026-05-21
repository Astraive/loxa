from __future__ import annotations

from datetime import datetime, timedelta
from typing import Any

from .event import Attr

def String(key: str, value: str) -> Attr: return Attr(key, value)
def Int(key: str, value: int) -> Attr: return Attr(key, value)
def Int64(key: str, value: int) -> Attr: return Attr(key, value)
def Uint64(key: str, value: int) -> Attr: return Attr(key, value)
def Float64(key: str, value: float) -> Attr: return Attr(key, value)
def Bool(key: str, value: bool) -> Attr: return Attr(key, value)
def Time(key: str, value: datetime) -> Attr: return Attr(key, value.isoformat())
def Duration(key: str, value: timedelta) -> Attr: return Attr(key, int(value.total_seconds() * 1000))
def Any(key: str, value: Any) -> Attr: return Attr(key, value)
def Null(key: str) -> Attr: return Attr(key, None)
def Group(key: str, *attrs: Attr) -> Attr:
    result: dict[str, object] = {}
    for a in attrs:
        _set_nested(result, a.key, a.value)
    return Attr(key, result)

def _set_nested(target: dict[str, object], key: str, value: object) -> None:
    if "." not in key:
        target[key] = value
        return
    parts = key.split(".")
    current = target
    for part in parts[:-1]:
        current = current.setdefault(part, {})  # type: ignore[arg-type]
    current[parts[-1]] = value
def SensitiveString(key: str, value: str) -> Attr: return Attr(key, value, sensitive=True)
def HashString(key: str, value: str) -> Attr: return Attr(key, value, hash_value=True)
def MarkSensitive(attr: Attr) -> Attr: return Attr(attr.key, attr.value, sensitive=True, hash_value=attr.hash_value, drop=attr.drop)
