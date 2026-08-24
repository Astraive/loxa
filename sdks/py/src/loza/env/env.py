"""Environment-backed configuration helpers."""

from __future__ import annotations

import os

from ..config.config import Config, _apply_env_vars


def get(key: str, default: str = "") -> str:
    """Return a trimmed environment value or ``default`` when unset."""
    return os.getenv(key, default).strip()


def bool_env(key: str, default: bool = False) -> bool:
    """Parse conventional boolean environment values."""
    value = get(key).lower()
    if value in {"1", "true", "yes", "on"}:
        return True
    if value in {"0", "false", "no", "off"}:
        return False
    return default


def int_env(key: str, default: int = 0) -> int:
    """Parse an integer environment value, returning ``default`` on failure."""
    try:
        return int(get(key))
    except ValueError:
        return default


def load_env_config() -> Config:
    """Build a configuration value from the current process environment."""
    return _apply_env_vars(Config())
