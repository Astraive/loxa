from __future__ import annotations

from typing import Any
from .duplicate_policy import CanonicalWins, ErrorOnDuplicate, FirstWins, KeepBoth, LastWins, UserWins


def apply_duplicate(target: dict[str, Any], key: str, value: Any, policy: str) -> None:
    if key not in target or policy in {UserWins, LastWins}:
        target[key] = value
        return
    if policy in {CanonicalWins, FirstWins}:
        return
    if policy == KeepBoth:
        old = target[key]
        target[key] = old + [value] if isinstance(old, list) else [old, value]
        return
    if policy == ErrorOnDuplicate:
        raise ValueError(f"duplicate field: {key}")
    target[key] = value
