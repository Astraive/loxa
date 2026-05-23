from __future__ import annotations

import random
import threading
import time
from datetime import timedelta
from collections.abc import Callable
from typing import Any

from .event import EventContext

Sampler = Callable[[EventContext], bool]


def sample_all() -> Sampler:
    return lambda event: True


def sample_none() -> Sampler:
    return lambda event: False


def sample_random(rate: float) -> Sampler:
    bounded = max(0.0, min(1.0, rate))
    return lambda event: random.random() < bounded


def sample_errors() -> Sampler:
    return lambda event: event.outcome == "error" or event.error is not None


def sample_slow_requests(duration: timedelta) -> Sampler:
    threshold = int(duration.total_seconds() * 1000)
    return lambda event: event.duration_ms() >= threshold


def sample_status_codes(*codes: int) -> Sampler:
    wanted = set(codes)
    return lambda event: event.params.status_code in wanted


def sample_routes(*routes: str) -> Sampler:
    wanted = set(routes)
    return lambda event: event.params.route in wanted or event.params.path in wanted


def sample_users(*ids: str) -> Sampler:
    wanted = set(ids)
    return lambda event: str(event.user.get("id", "")) in wanted


def sample_tenants(*ids: str) -> Sampler:
    wanted = set(ids)
    return lambda event: str(event.tenant.get("id", "")) in wanted


def sample_feature_flag(name: str, value: Any) -> Sampler:
    return lambda event: isinstance(event.attrs.get("feature"), dict) and event.attrs["feature"].get(name) == value


def sample_by_header(header: str, value: str) -> Sampler:
    wanted_header = header.lower()
    return lambda event: str(event.attrs.get(f"http.request.header.{wanted_header}", "")) == value


def any_sampler(*samplers: Sampler) -> Sampler:
    return lambda event: any(s(event) for s in samplers)


def all_sampler(*samplers: Sampler) -> Sampler:
    return lambda event: all(s(event) for s in samplers)


def not_sampler(sampler: Sampler) -> Sampler:
    return lambda event: not sampler(event)


class _RateLimitedSampler:
    """Token-bucket rate limiter sampler."""

    def __init__(self, rate: float, window: float) -> None:
        self.rate = rate
        self.window = window
        self._lock = threading.Lock()
        self._tokens = rate
        self._last = time.monotonic()

    def __call__(self, event: EventContext) -> bool:
        with self._lock:
            now = time.monotonic()
            elapsed = now - self._last
            self._last = now
            capacity = max(1.0, self.rate)
            self._tokens += elapsed * (self.rate / max(self.window, 0.001))
            if self._tokens > capacity:
                self._tokens = capacity
            if self._tokens < 1:
                return False
            self._tokens -= 1
            return True


def sample_rate_limited(rate: float, window: float = 1.0) -> Sampler:
    """Keep at most `rate` events per `window` seconds using a token-bucket strategy."""
    if rate <= 0 or window <= 0:
        return sample_none()
    return _RateLimitedSampler(rate, window)


def sample_by_event(fn: Callable[[EventContext], bool]) -> Sampler:
    return fn


def sample_by_outcome(*outcomes: str) -> Sampler:
    wanted = set(outcomes)
    return lambda event: event.outcome in wanted


def should_sample(sampler: Sampler, event: EventContext) -> bool:
    return sampler(event)


def allow_fields(*keys: str) -> Sampler:
    allowed = set(keys)
    return lambda event: all(k in event.attrs for k in allowed)


def block_fields(*keys: str) -> Sampler:
    blocked = set(keys)
    return lambda event: not any(k in event.attrs for k in blocked)


# PascalCase aliases
SampleByEvent = sample_by_event
SampleByOutcome = sample_by_outcome
ShouldSample = should_sample
AllowFields = allow_fields
BlockFields = block_fields
