from __future__ import annotations

import os
from pathlib import Path
from dataclasses import dataclass, field, replace
from typing import Any, Callable, Protocol, runtime_checkable
from urllib.parse import quote

from ..sinks.stdout import StdoutSink
from ..generated.spec_contract import LOZA_EVENT_VERSION, LOZA_INGEST_API_VERSION, LOZA_SPEC_VERSION

__all__ = [
    "CanonicalWins",
    "UserWins",
    "FirstWins",
    "LastWins",
    "KeepBoth",
    "ErrorOnDuplicate",
    "ExpandDotKeys",
    "PreserveDotKeys",
    "SnakeCaseKeys",
    "CamelCaseKeys",
    "StatsHandler",
    "DeliveryFailureHandler",
    "AsyncConfig",
    "SecurityConfig",
    "FieldNamingConfig",
    "Config",
    "LOZA_EVENT_VERSION",
    "LOZA_INGEST_API_VERSION",
    "LOZA_SPEC_VERSION",
    "load_layered_config",
    "new_client",
]

CanonicalWins = "canonical_wins"
UserWins = "user_wins"
FirstWins = "first_wins"
LastWins = "last_wins"
KeepBoth = "keep_both"
ErrorOnDuplicate = "error_on_duplicate"

ExpandDotKeys = "expand_dot_keys"
PreserveDotKeys = "preserve_dot_keys"
SnakeCaseKeys = "snake_case_keys"
CamelCaseKeys = "camel_case_keys"


@runtime_checkable
class StatsHandler(Protocol):
    """Receives logger pipeline telemetry callbacks."""

    def on_emit(self, event: Any) -> None: ...
    def on_drop(self, reason: str) -> None: ...
    def on_error(self, error: Exception) -> None: ...
    def on_collector_ack(self, acks: list, errors: list, request_id: str, deduped: int) -> None: ...


@runtime_checkable
class DeliveryFailureHandler(StatsHandler, Protocol):
    """Optional extension for explicit delivery-failure callbacks."""

    def on_delivery_failed(self, event: Any, error: Exception) -> None: ...


@dataclass(slots=True)
class AsyncConfig:
    enabled: bool = False
    queue_size: int = 8192
    workers: int = 1
    max_batch_bytes: int = 256 * 1024
    flush_interval_ms: int = 5000
    batch_size: int = 100


@dataclass(slots=True)
class SecurityConfig:
    redact_by_default: bool = True
    allow_pii: bool = False
    max_field_bytes: int = 4096
    max_event_bytes: int = 256 * 1024
    max_attr_count: int = 512
    drop_oversized_events: bool = True


@dataclass(slots=True)
class FieldNamingConfig:
    policy: str = ExpandDotKeys
    custom_mapper: Callable[[str], str] | None = None


@dataclass(slots=True)
class Config:
    service: str = ""
    alias: str = ""
    version: str = ""
    environment: str = "development"
    region: str = ""
    level: str = "info"
    strict: bool = False
    sinks: list[Any] = field(default_factory=list)
    collector_endpoint: str = ""
    collector_name: str = ""
    api_key: str = ""
    username: str = field(default="", repr=False)
    password: str = field(default="", repr=False)
    duplicate_policy: str = "canonical_wins"
    schema: Any = None
    sampler: Any = None
    redactor: Any = None
    metrics: Any = None
    stats_handler: Any = None
    async_config: AsyncConfig = field(default_factory=AsyncConfig)
    security: SecurityConfig = field(default_factory=SecurityConfig)
    field_naming: FieldNamingConfig = field(default_factory=FieldNamingConfig)
    checkpoint_emit_immediately: bool = False
    deployment_id: str = ""
    include_host: bool = False
    max_checkpoints: int = 32
    panic_recovery: bool = False
    exit_on_fatal: bool = False
    release: str = ""
    namespace: str = ""
    otel_bridge: bool = False
    retry: bool = False
    timeout: float = 0.0
    logger: Any = None

    @classmethod
    def dev(cls, service: str = "") -> "Config":
        return cls(service=service, environment="development", level="debug", sinks=[StdoutSink()])

    @classmethod
    def production(cls, service: str = "") -> "Config":
        return cls(service=service, environment="production", level="info", strict=True, sinks=[StdoutSink()])

    @classmethod
    def test(cls, service: str = "") -> "Config":
        return cls(service=service, environment="test", level="debug", strict=True, sinks=[])

    def with_service(self, service: str) -> "Config":
        return replace(self, service=service)

    def with_alias(self, alias: str) -> "Config":
        return replace(self, alias=alias)

    def with_version(self, version: str) -> "Config":
        return replace(self, version=version)

    def with_environment(self, environment: str) -> "Config":
        return replace(self, environment=environment)

    def with_region(self, region: str) -> "Config":
        return replace(self, region=region)

    def with_sink(self, sink: Any) -> "Config":
        return replace(self, sinks=[*self.sinks, sink])

    def with_sampler(self, sampler: Any) -> "Config":
        return replace(self, sampler=sampler)

    def with_redactor(self, redactor: Any) -> "Config":
        return replace(self, redactor=redactor)

    def with_metrics(self, metrics: Any) -> "Config":
        return replace(self, metrics=metrics)

    def with_schema(self, schema: Any) -> "Config":
        return replace(self, schema=schema)

    def with_event_schema(self, schema: Any) -> "Config":
        return self.with_schema(schema)

    def with_async(self, enabled: bool) -> "Config":
        return replace(self, async_config=replace(self.async_config, enabled=enabled))

    def with_collector_endpoint(self, endpoint: str) -> "Config":
        return replace(self, collector_endpoint=endpoint)

    def with_collector_name(self, collector_name: str) -> "Config":
        return replace(self, collector_name=collector_name)

    def with_basic_auth(self, username: str, password: str) -> "Config":
        return replace(self, username=username, password=password)

    def with_api_key(self, api_key: str) -> "Config":
        return replace(self, api_key=api_key.strip())

    def with_duplicate_policy(self, policy: str) -> "Config":
        return replace(self, duplicate_policy=policy)

    def with_exit_on_fatal(self, enabled: bool = True) -> "Config":
        return replace(self, exit_on_fatal=enabled)

    def with_release(self, value: str) -> "Config":
        return replace(self, release=value)

    def with_namespace(self, value: str) -> "Config":
        return replace(self, namespace=value)

    def with_otel_bridge(self, value: bool) -> "Config":
        async_config = self.async_config
        if value:
            async_config = replace(async_config, enabled=True)
        return replace(self, otel_bridge=value, async_config=async_config)

    def with_retry(self, value: bool) -> "Config":
        return replace(self, retry=value)

    def with_timeout(self, value: float) -> "Config":
        return replace(self, timeout=value)

    def with_logger(self, value: Any) -> "Config":
        return replace(self, logger=value)

    @classmethod
    def disabled(cls) -> "Config":
        cfg = cls(environment="test", level="fatal", strict=False)
        return cfg

    @classmethod
    def from_env(cls) -> "Config":
        from .config import _apply_env_vars

        return _apply_env_vars(cls())

    def validate(self) -> None:
        if self.collector_endpoint.strip().startswith("loza://"):
            _apply_dsn(self, self.collector_endpoint.strip(), include_credentials=not bool(self.api_key))
        if self.level not in {"debug", "info", "notice", "warn", "error", "fatal"}:
            raise ValueError(f"unsupported level: {self.level}")
        if self.async_config.queue_size <= 0:
            raise ValueError("async queue_size must be positive")
        if self.async_config.workers <= 0:
            raise ValueError("async workers must be positive")
        if self.security.max_event_bytes <= 0:
            raise ValueError("max_event_bytes must be positive")
        if self.strict and not self.service:
            raise ValueError("strict mode requires service")
        from .dsn import is_public_dsn_username

        if not self.username and self.password:
            raise ValueError("collector basic auth password requires a username")
        if self.username and not self.password and not is_public_dsn_username(self.username):
            raise ValueError("collector basic auth requires a password unless username is an lx_pub_ capability")
        if self.username and self.collector_endpoint.lower().startswith("http://"):
            from urllib.parse import urlparse

            host = (urlparse(self.collector_endpoint).hostname or "").lower()
            if host not in {"localhost", "127.0.0.1", "::1"}:
                raise ValueError("collector basic auth requires HTTPS except for local endpoints")


def load_layered_config() -> Config:
    raw: dict[str, Any] = {}
    defaults_path = _find_defaults_path()
    raw = _merge_dicts(raw, _parse_simple_yaml(defaults_path.read_text(encoding="utf-8")))
    user_path = _find_user_config_path()
    if user_path is not None and user_path.exists():
        raw = _merge_dicts(raw, _parse_simple_yaml(user_path.read_text(encoding="utf-8")))
    return _config_from_mapping(raw)


def new_client(code_config: Config):  # -> Logger
    """Create a Logger with 4-layer config precedence: defaults -> file -> env -> code."""
    # Step 1: Start with hardcoded defaults
    base = Config()

    # Step 2: Load defaults YAML + user YAML
    defaults_raw: dict[str, Any] = {}
    defaults_path = _find_defaults_path()
    if defaults_path.exists():
        defaults_raw = _parse_simple_yaml(defaults_path.read_text(encoding="utf-8"))
    user_path = _find_user_config_path()
    if user_path is not None and user_path.exists():
        defaults_raw = _merge_dicts(defaults_raw, _parse_simple_yaml(user_path.read_text(encoding="utf-8")))

    # Step 3: Apply file config to base (only non-empty values)
    file_cfg = _config_from_mapping(defaults_raw)
    merged = _merge_file_config(base, file_cfg)

    # Step 4: Apply env vars (override file config)
    merged = _apply_env_vars(merged)

    # Step 5: Apply code config (highest precedence)
    merged = _merge_code_config(merged, code_config)

    # Step 6: Validate
    merged.validate()

    from .logger import Logger

    return Logger(merged)


def _collector_ingest_endpoint(endpoint: str, collector_name: str = "") -> str:
    endpoint = endpoint.strip().rstrip("/")
    if collector_name:
        return f"{endpoint}/collectors/{quote(collector_name, safe='')}/events"
    if endpoint.endswith("/events"):
        return endpoint
    return f"{endpoint}/events"


def _apply_dsn(cfg: Config, raw: str, *, include_credentials: bool = True) -> None:
    from .dsn import parse

    dsn = parse(raw)
    cfg.collector_endpoint = dsn.base_url
    cfg.collector_name = dsn.collector_name
    if dsn.env != "default":
        cfg.environment = dsn.env
    if dsn.service:
        cfg.service = dsn.service
    if include_credentials and dsn.username and not cfg.api_key and not cfg.username:
        cfg.username = dsn.username
        cfg.password = dsn.password


def _resolve_endpoint_source(cfg: Config, raw: str, *, include_credentials: bool) -> None:
    raw = raw.strip()
    if raw.startswith("loza://"):
        _apply_dsn(cfg, raw, include_credentials=include_credentials)
    else:
        cfg.collector_endpoint = raw


def _apply_env_vars(cfg: Config) -> Config:
    """Apply environment variables to config, overriding file values."""
    dsn_raw = os.getenv("LOZA_DSN", "").strip()
    if dsn_raw:
        _resolve_endpoint_source(cfg, dsn_raw, include_credentials=True)

    collector_url = os.getenv("LOZA_COLLECTOR_URL", "").strip()
    collector_endpoint = os.getenv("LOZA_COLLECTOR_ENDPOINT", "").strip()
    if collector_url:
        _resolve_endpoint_source(cfg, collector_url, include_credentials=False)
    elif collector_endpoint:
        _resolve_endpoint_source(cfg, collector_endpoint, include_credentials=False)

    env_map = {
        "LOZA_SERVICE": "service",
        "LOZA_SERVICE_NAME": "service",
        "LOZA_SERVICE_VERSION": "version",
        "LOZA_ENVIRONMENT": "environment",
        "LOZA_REGION": "region",
        "LOZA_LOG_LEVEL": "level",
        "LOZA_API_KEY": "api_key",
        "LOZA_DUPLICATE_POLICY": "duplicate_policy",
    }
    for env_key, cfg_field in env_map.items():
        val = os.getenv(env_key, "").strip()
        if val:
            setattr(cfg, cfg_field, val)

    # Strict mode from env
    strict_env = os.getenv("LOZA_STRICT", "").strip().lower()
    if strict_env in ("1", "true", "yes"):
        cfg.strict = True
    elif strict_env in ("0", "false", "no"):
        cfg.strict = False

    # Async from env
    async_env = os.getenv("LOZA_ASYNC_ENABLED", "").strip().lower()
    if async_env in ("1", "true", "yes"):
        cfg.async_config.enabled = True
    elif async_env in ("0", "false", "no"):
        cfg.async_config.enabled = False

    # Async queue size
    queue_env = os.getenv("LOZA_MAX_BUFFER_SIZE", "").strip()
    if queue_env.isdigit():
        cfg.async_config.queue_size = int(queue_env)

    # Batch size (event count)
    batch_env = os.getenv("LOZA_BATCH_SIZE", "").strip()
    if batch_env.isdigit():
        cfg.async_config.batch_size = int(batch_env)

    # Max event bytes
    max_bytes_env = os.getenv("LOZA_MAX_EVENT_BYTES", "").strip()
    if max_bytes_env.isdigit():
        cfg.security.max_event_bytes = int(max_bytes_env)

    return cfg


def _merge_file_config(base: Config, file_cfg: Config) -> Config:
    """Apply file config values only when base has defaults (empty strings, default values)."""
    if file_cfg.service and not base.service:
        base.service = file_cfg.service
    if file_cfg.version and not base.version:
        base.version = file_cfg.version
    if file_cfg.environment and file_cfg.environment != "development":
        base.environment = file_cfg.environment
    if file_cfg.region and not base.region:
        base.region = file_cfg.region
    if file_cfg.level and file_cfg.level != "info":
        base.level = file_cfg.level
    if file_cfg.strict:
        base.strict = file_cfg.strict
    if file_cfg.collector_endpoint and not base.collector_endpoint:
        _resolve_endpoint_source(base, file_cfg.collector_endpoint, include_credentials=True)
    if file_cfg.collector_name and not base.collector_name:
        base.collector_name = file_cfg.collector_name
    if file_cfg.api_key and not base.api_key:
        base.api_key = file_cfg.api_key
    if file_cfg.username and not base.username and not base.api_key:
        base.username = file_cfg.username
        base.password = file_cfg.password
    if file_cfg.duplicate_policy and file_cfg.duplicate_policy != CanonicalWins:
        base.duplicate_policy = file_cfg.duplicate_policy
    if file_cfg.checkpoint_emit_immediately:
        base.checkpoint_emit_immediately = file_cfg.checkpoint_emit_immediately
    return base


def _merge_code_config(base: Config, code: Config) -> Config:
    """Apply code config values (highest precedence). Non-default values override."""
    if code.service:
        base.service = code.service
    if code.version:
        base.version = code.version
    if code.environment and code.environment != "development":
        base.environment = code.environment
    if code.region:
        base.region = code.region
    if code.level and code.level != "info":
        base.level = code.level
    if code.strict:
        base.strict = code.strict
    if code.collector_endpoint:
        _resolve_endpoint_source(base, code.collector_endpoint, include_credentials=not bool(code.api_key))
    if code.api_key:
        base.api_key = code.api_key.strip()
    if code.collector_name:
        base.collector_name = code.collector_name
    if code.username and not code.api_key:
        base.username = code.username
        base.password = code.password
    if code.duplicate_policy and code.duplicate_policy != CanonicalWins:
        base.duplicate_policy = code.duplicate_policy
    if code.sinks:
        base.sinks = code.sinks
    if code.sampler is not None:
        base.sampler = code.sampler
    if code.redactor is not None:
        base.redactor = code.redactor
    if code.metrics is not None:
        base.metrics = code.metrics
    if code.stats_handler is not None:
        base.stats_handler = code.stats_handler
    if code.schema is not None:
        base.schema = code.schema
    if code.checkpoint_emit_immediately:
        base.checkpoint_emit_immediately = code.checkpoint_emit_immediately
    if code.async_config.enabled:
        base.async_config.enabled = code.async_config.enabled
    if code.async_config.queue_size != 8192:
        base.async_config.queue_size = code.async_config.queue_size
    if code.async_config.workers != 1:
        base.async_config.workers = code.async_config.workers
    return base


def _find_defaults_path() -> Path:
    override = os.getenv("LOZA_PY_DEFAULTS", "").strip()
    if override:
        return Path(override)
    here = Path(__file__).resolve()
    candidates = [
        here.parents[1] / "loza-py.defaults.yaml",
        here.parents[2] / "loza-py.defaults.yaml",
        here.parents[3] / "loza-py.defaults.yaml",
        Path.cwd() / "loza-py.defaults.yaml",
    ]
    for candidate in candidates:
        if candidate.exists():
            return candidate
    return candidates[0]


def _find_user_config_path() -> Path | None:
    override = os.getenv("LOZA_PY_CONFIG", "").strip()
    if override:
        return Path(override)
    for candidate in (Path.cwd() / ".loza-py.yaml", Path.cwd() / "loza.yaml"):
        if candidate.exists():
            return candidate
    return None


def _config_from_mapping(data: dict[str, Any]) -> Config:
    async_map = data.get("async_config") if isinstance(data.get("async_config"), dict) else {}
    security_map = data.get("security") if isinstance(data.get("security"), dict) else {}
    field_map = data.get("field_naming") if isinstance(data.get("field_naming"), dict) else {}
    return Config(
        service=str(data.get("service", "")),
        version=str(data.get("version", "")),
        environment=str(data.get("environment", "development")),
        region=str(data.get("region", "")),
        level=str(data.get("level", "info")),
        strict=bool(data.get("strict", False)),
        collector_endpoint=str(data.get("collector_endpoint", "")),
        collector_name=str(data.get("collector_name", "")),
        api_key=str(data.get("api_key", "")),
        username=str(data.get("username", "")),
        password=str(data.get("password", "")),
        duplicate_policy=str(data.get("duplicate_policy", CanonicalWins)),
        checkpoint_emit_immediately=bool(data.get("checkpoint_emit_immediately", False)),
        async_config=AsyncConfig(
            enabled=bool(async_map.get("enabled", False)),
            queue_size=int(async_map.get("queue_size", 8192)),
            workers=int(async_map.get("workers", 1)),
            max_batch_bytes=int(async_map.get("max_batch_bytes", 256 * 1024)),
        ),
        security=SecurityConfig(
            redact_by_default=bool(security_map.get("redact_by_default", True)),
            allow_pii=bool(security_map.get("allow_pii", False)),
            max_field_bytes=int(security_map.get("max_field_bytes", 4096)),
            max_event_bytes=int(security_map.get("max_event_bytes", 256 * 1024)),
            max_attr_count=int(security_map.get("max_attr_count", 512)),
            drop_oversized_events=bool(security_map.get("drop_oversized_events", True)),
        ),
        field_naming=FieldNamingConfig(
            policy=str(field_map.get("policy", ExpandDotKeys)),
            custom_mapper=None,
        ),
    )


def _merge_dicts(base: dict[str, Any], override: dict[str, Any]) -> dict[str, Any]:
    merged = dict(base)
    for k, v in override.items():
        if isinstance(v, dict) and isinstance(merged.get(k), dict):
            merged[k] = _merge_dicts(merged[k], v)
        else:
            merged[k] = v
    return merged


def _parse_simple_yaml(content: str) -> dict[str, Any]:
    root: dict[str, Any] = {}
    stack: list[tuple[int, dict[str, Any]]] = [(-1, root)]
    for raw_line in content.splitlines():
        if not raw_line.strip() or raw_line.lstrip().startswith("#"):
            continue
        indent = len(raw_line) - len(raw_line.lstrip(" "))
        line = raw_line.split("#", 1)[0].rstrip()
        if ":" not in line:
            continue
        key, raw_value = line.lstrip().split(":", 1)
        key = key.strip()
        value = raw_value.strip()
        while len(stack) > 1 and indent <= stack[-1][0]:
            stack.pop()
        current = stack[-1][1]
        if value == "":
            child: dict[str, Any] = {}
            current[key] = child
            stack.append((indent, child))
            continue
        current[key] = _parse_scalar(value)
    return root


def _parse_scalar(value: str) -> Any:
    if len(value) >= 2 and value[0] == value[-1] and value[0] in {"'", '"'}:
        value = value[1:-1]
    lower = value.lower()
    if lower in {"true", "false"}:
        return lower == "true"
    if value.isdigit():
        return int(value)
    return value
