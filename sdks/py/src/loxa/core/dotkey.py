from __future__ import annotations

from typing import Any

def set_path(target: dict[str, Any], key: str, value: Any) -> None:
    parts = key.split('.') if key else []
    cur = target
    for part in parts[:-1]:
        child = cur.get(part)
        if not isinstance(child, dict):
            child = {}
            cur[part] = child
        cur = child
    if parts:
        cur[parts[-1]] = value

def get_path(target: dict[str, Any], key: str) -> Any:
    cur: Any = target
    for part in key.split('.'):
        if not isinstance(cur, dict) or part not in cur:
            return None
        cur = cur[part]
    return cur

def delete_path(target: dict[str, Any], key: str) -> None:
    parts = key.split('.')
    cur = target
    for part in parts[:-1]:
        child = cur.get(part)
        if not isinstance(child, dict):
            return
        cur = child
    cur.pop(parts[-1], None)
