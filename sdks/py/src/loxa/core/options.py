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
