from __future__ import annotations

import re
from typing import Iterable


CONTROL_CHARS = re.compile(r"[\x00-\x08\x0b\x0c\x0e-\x1f]")


def quote(value: str) -> str:
    return value.encode("unicode_escape").decode("ascii")


def scrub_control_chars(value: str) -> str:
    return CONTROL_CHARS.sub("", value)


def truncate_utf8(value: str, max_bytes: int) -> str:
    encoded = value.encode("utf-8")
    if len(encoded) <= max_bytes:
        return value
    return encoded[:max_bytes].decode("utf-8", errors="ignore")


def join_path(parts: Iterable[str]) -> str:
    return ".".join(part.strip(".") for part in parts if part)
