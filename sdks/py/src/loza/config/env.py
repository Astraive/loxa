"""Environment variable handling for configuration."""

from __future__ import annotations

import os

ENV_SERVICE_NAME = "LOZA_SERVICE_NAME"
ENV_SERVICE_VERSION = "LOZA_SERVICE_VERSION"
ENV_ENVIRONMENT = "LOZA_ENVIRONMENT"
ENV_REGION = "LOZA_REGION"
ENV_LOG_LEVEL = "LOZA_LOG_LEVEL"
ENV_COLLECTOR_URL = "LOZA_COLLECTOR_URL"
ENV_COLLECTOR_ENDPOINT = "LOZA_COLLECTOR_ENDPOINT"
ENV_DUPLICATE_POLICY = "LOZA_DUPLICATE_POLICY"
ENV_STRICT = "LOZA_STRICT"
ENV_ASYNC_ENABLED = "LOZA_ASYNC_ENABLED"
ENV_MAX_BUFFER_SIZE = "LOZA_MAX_BUFFER_SIZE"
ENV_BATCH_SIZE = "LOZA_BATCH_SIZE"
ENV_MAX_EVENT_BYTES = "LOZA_MAX_EVENT_BYTES"
ENV_COLLECTOR_API_KEY = "LOZA_COLLECTOR_API_KEY"
ENV_COLLECTOR_API_KEY_HEADER = "LOZA_COLLECTOR_API_KEY_HEADER"


def get_env(key: str, default: str = "") -> str:
    return os.getenv(key, default).strip()


def get_env_bool(key: str, default: bool = False) -> bool:
    val = get_env(key).lower()
    if val in ("1", "true", "yes"):
        return True
    if val in ("0", "false", "no"):
        return False
    return default


def get_env_int(key: str, default: int = 0) -> int:
    val = get_env(key)
    return int(val) if val.isdigit() else default
