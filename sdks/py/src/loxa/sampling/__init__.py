from __future__ import annotations

import random
from datetime import timedelta
from typing import Callable, Any

from ..core.event import EventContext

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


def any_sampler(*samplers: Sampler) -> Sampler:
    return lambda event: any(s(event) for s in samplers)


def all_sampler(*samplers: Sampler) -> Sampler:
    return lambda event: all(s(event) for s in samplers)


def not_sampler(sampler: Sampler) -> Sampler:
    return lambda event: not sampler(event)
