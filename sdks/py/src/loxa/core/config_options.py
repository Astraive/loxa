from __future__ import annotations

from typing import Callable, Any
from .config import Config

ConfigOption = Callable[[Config], Config]

def WithService(value: str) -> ConfigOption: return lambda cfg: cfg.with_service(value)
def WithVersion(value: str) -> ConfigOption: return lambda cfg: cfg.with_version(value)
def WithEnvironment(value: str) -> ConfigOption: return lambda cfg: cfg.with_environment(value)
def WithSink(value: Any) -> ConfigOption: return lambda cfg: cfg.with_sink(value)
def WithSampler(value: Any) -> ConfigOption: return lambda cfg: cfg.with_sampler(value)
def WithRedactor(value: Any) -> ConfigOption: return lambda cfg: cfg.with_redactor(value)
def WithMetrics(value: Any) -> ConfigOption: return lambda cfg: cfg.with_metrics(value)
def WithSchema(value: Any) -> ConfigOption: return lambda cfg: cfg.with_schema(value)
def WithEventSchema(value: Any) -> ConfigOption: return lambda cfg: cfg.with_event_schema(value)
def WithAsync(value: bool) -> ConfigOption: return lambda cfg: cfg.with_async(value)
def WithCollectorEndpoint(value: str) -> ConfigOption: return lambda cfg: cfg.with_collector_endpoint(value)
def WithDuplicatePolicy(value: str) -> ConfigOption: return lambda cfg: cfg.with_duplicate_policy(value)
def WithStatsHandler(value: Any) -> ConfigOption: return lambda cfg: setattr(cfg, "stats_handler", value) or cfg
def WithDeploymentID(value: str) -> ConfigOption: return lambda cfg: setattr(cfg, "deployment_id", value) or cfg
def WithIncludeHost(value: bool) -> ConfigOption: return lambda cfg: setattr(cfg, "include_host", value) or cfg
def WithPanicRecovery(value: bool) -> ConfigOption: return lambda cfg: setattr(cfg, "panic_recovery", value) or cfg
def WithExitOnFatal(enabled: bool = True) -> ConfigOption: return lambda cfg: cfg.with_exit_on_fatal(enabled)
def WithRelease(value: str) -> ConfigOption: return lambda cfg: cfg.with_release(value)
def WithNamespace(value: str) -> ConfigOption: return lambda cfg: cfg.with_namespace(value)
def WithApiKey(value: str) -> ConfigOption: return lambda cfg: cfg.with_api_key(value)
def WithOtelBridge(value: bool) -> ConfigOption: return lambda cfg: cfg.with_otel_bridge(value)
def WithRetry(value: bool) -> ConfigOption: return lambda cfg: cfg.with_retry(value)
def WithTimeout(value: float) -> ConfigOption: return lambda cfg: cfg.with_timeout(value)
def WithQueueSize(value: int) -> ConfigOption: return lambda cfg: setattr(cfg.async_config, "queue_size", value) or cfg
def WithFlushInterval(value: int) -> ConfigOption: return lambda cfg: setattr(cfg.async_config, "flush_interval_ms", value) or cfg
def WithBatchSize(value: int) -> ConfigOption: return lambda cfg: setattr(cfg.async_config, "batch_size", value) or cfg
def WithLogger(value: Any) -> ConfigOption: return lambda cfg: cfg.with_logger(value)
def Disabled() -> Config:
    return Config.disabled()
def FromEnv() -> Config:
    from .config import _apply_env_vars
    return _apply_env_vars(Config())

# --- snake_case aliases ---
with_service = WithService
with_version = WithVersion
with_environment = WithEnvironment
with_sink = WithSink
with_sampler = WithSampler
with_redactor = WithRedactor
with_metrics = WithMetrics
with_schema = WithSchema
with_event_schema = WithEventSchema
with_async = WithAsync
with_collector_endpoint = WithCollectorEndpoint
with_duplicate_policy = WithDuplicatePolicy
with_stats_handler = WithStatsHandler
with_deployment_id = WithDeploymentID
with_include_host = WithIncludeHost
with_panic_recovery = WithPanicRecovery
with_exit_on_fatal = WithExitOnFatal
with_release = WithRelease
with_namespace = WithNamespace
with_api_key = WithApiKey
with_otel_bridge = WithOtelBridge
with_retry = WithRetry
with_timeout = WithTimeout
with_queue_size = WithQueueSize
with_flush_interval = WithFlushInterval
with_batch_size = WithBatchSize
with_logger = WithLogger
disabled = Disabled
from_env = FromEnv
