from __future__ import annotations

import os
from dataclasses import dataclass


def get(name: str, default: str = "") -> str:
    return os.getenv(name, default)


def bool_env(name: str, default: bool = False) -> bool:
    value = os.getenv(name)
    if value is None:
        return default
    return value.lower() in {"1", "true", "yes", "on"}


def int_env(name: str, default: int = 0) -> int:
    try:
        return int(os.getenv(name, str(default)))
    except ValueError:
        return default


@dataclass(slots=True)
class EnvConfig:
    service: str = ""
    environment: str = ""
    collector_endpoint: str = ""
    api_key: str = ""


def load_env_config(prefix: str = "LOXA_") -> EnvConfig:
    return EnvConfig(
        service=get(prefix + "SERVICE"),
        environment=get(prefix + "ENVIRONMENT"),
        collector_endpoint=get(prefix + "COLLECTOR_ENDPOINT"),
        api_key=get(prefix + "API_KEY"),
    )
