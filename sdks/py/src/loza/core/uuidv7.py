from __future__ import annotations

import time
from uuid import uuid4
from typing import Callable

_custom_id_generator: Callable[[str], str] | None = None


def set_id_generator(fn: Callable[[str], str] | None) -> None:
    global _custom_id_generator
    _custom_id_generator = fn


def reset_id_generator() -> None:
    global _custom_id_generator
    _custom_id_generator = None


def uuidv7_like(prefix: str = "evt") -> str:
    if _custom_id_generator is not None:
        return _custom_id_generator(prefix)
    return f"{prefix}_{int(time.time() * 1000):x}_{uuid4().hex}"
