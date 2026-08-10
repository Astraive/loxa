from __future__ import annotations

import copy
import hashlib
import json
import re
from collections.abc import Callable
from typing import Any

REDACTED_VALUE = "[REDACTED]"

Redactor = Callable[[dict[str, Any]], dict[str, Any]]

# Safety-net keys for obviously sensitive fields. The collector owns the real
# PII policy (email, phone, SSN, IP, regex, tenant-specific rules, etc.).
DEFAULT_KEYS = {
    "password",
    "passwd",
    "pwd",
    "secret",
    "token",
    "access_token",
    "refresh_token",
    "api_key",
    "apikey",
    "auth",
    "authorization",
    "credential",
    "private_key",
    "client_secret",
}


def _walk(value: Any, fn: Callable[[str, Any], tuple[bool, Any]]) -> Any:
    if isinstance(value, dict):
        out = {}
        for key, child in value.items():
            keep, new_value = fn(key, child)
            if keep:
                out[key] = _walk(new_value, fn)
        return out
    if isinstance(value, list):
        return [_walk(item, fn) for item in value]
    return value


def _stable_text(value: Any) -> str:
    if isinstance(value, (dict, list, tuple)):
        try:
            return json.dumps(value, sort_keys=True, default=str, separators=(",", ":"))
        except TypeError:
            return str(value)
    return str(value)


def _hash(value: Any) -> str:
    return hashlib.sha256(_stable_text(value).encode("utf-8")).hexdigest()


def _mask(value: Any, prefix: int = 2, suffix: int = 2) -> str:
    text = _stable_text(value)
    if len(text) <= prefix + suffix:
        return "*" * max(4, len(text))
    return text[:prefix] + "*" * (len(text) - prefix - suffix) + text[-suffix:]


def _matches_key(key: str, wanted: set[str]) -> bool:
    lowered = key.lower()
    pieces = [lowered, lowered.split(".")[-1], lowered.split("_")[-1], lowered.split("-")[-1]]
    return any(piece in wanted for piece in pieces)


def redact(*keys: str) -> Redactor:
    """Alias for redact_keys."""
    return redact_keys(*keys)


def redact_keys(*keys: str) -> Redactor:
    wanted = {key.lower() for key in keys}
    return lambda payload: _walk(
        copy.deepcopy(payload),
        lambda key, value: (True, REDACTED_VALUE) if _matches_key(key, wanted) else (True, value),
    )


def default_redactor() -> Redactor:
    return redact_keys(*DEFAULT_KEYS)


def hash_keys(*keys: str) -> Redactor:
    wanted = {key.lower() for key in keys}
    return lambda payload: _walk(
        copy.deepcopy(payload),
        lambda key, value: (True, _hash(value)) if _matches_key(key, wanted) else (True, value),
    )


def drop_keys(*keys: str) -> Redactor:
    wanted = {key.lower() for key in keys}
    return lambda payload: _walk(
        copy.deepcopy(payload),
        lambda key, value: (False, value) if _matches_key(key, wanted) else (True, value),
    )


def mask_keys(*keys: str, prefix: int = 2, suffix: int = 2) -> Redactor:
    wanted = {key.lower() for key in keys}
    return lambda payload: _walk(
        copy.deepcopy(payload),
        lambda key, value: (
            (True, _mask(value, prefix=prefix, suffix=suffix)) if _matches_key(key, wanted) else (True, value)
        ),
    )


def redact_patterns(*patterns: str) -> Redactor:
    """Redact string values matching any of the given regex patterns."""
    compiled = [re.compile(p) for p in patterns]

    def _apply(payload: dict[str, Any]) -> dict[str, Any]:
        def _check(key: str, value: Any) -> tuple[bool, Any]:
            if isinstance(value, str):
                for pat in compiled:
                    if pat.search(value):
                        return True, REDACTED_VALUE
            return True, value

        return _walk(copy.deepcopy(payload), _check)

    return _apply


def sensitive_attrs_redactor(sensitive_keys: set[str]) -> Redactor:
    """Create a redactor that redacts keys marked sensitive via Attr.sensitive."""
    if not sensitive_keys:
        return lambda payload: payload
    return redact_keys(*sensitive_keys)


def compose_redactors(*redactors: Redactor) -> Redactor:
    def apply(payload: dict[str, Any]) -> dict[str, Any]:
        current = copy.deepcopy(payload)
        for redactor in redactors:
            current = redactor(current)
        return current

    return apply
