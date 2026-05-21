"""Environment variable handling for configuration."""

from __future__ import annotations

import os
from typing import Any

ENV_SERVICE_NAME = "LOXA_SERVICE_NAME"
ENV_SERVICE_VERSION = "LOXA_SERVICE_VERSION"
ENV_ENVIRONMENT = "LOXA_ENVIRONMENT"
ENV_REGION = "LOXA_REGION"
ENV_LOG_LEVEL = "LOXA_LOG_LEVEL"
ENV_COLLECTOR_URL = "LOXA_COLLECTOR_URL"
ENV_COLLECTOR_ENDPOINT = "LOXA_COLLECTOR_ENDPOINT"
ENV_DUPLICATE_POLICY = "LOXA_DUPLICATE_POLICY"
ENV_STRICT = "LOXA_STRICT"
ENV_ASYNC_ENABLED = "LOXA_ASYNC_ENABLED"
ENV_MAX_BUFFER_SIZE = "LOXA_MAX_BUFFER_SIZE"
ENV_BATCH_SIZE = "LOXA_BATCH_SIZE"
ENV_MAX_EVENT_BYTES = "LOXA_MAX_EVENT_BYTES"
ENV_COLLECTOR_API_KEY = "LOXA_COLLECTOR_API_KEY"
ENV_COLLECTOR_API_KEY_HEADER = "LOXA_COLLECTOR_API_KEY_HEADER"


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
