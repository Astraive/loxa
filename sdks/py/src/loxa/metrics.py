from __future__ import annotations

from collections import defaultdict
from dataclasses import dataclass, field
from datetime import timedelta
from threading import Lock
from typing import Any, Iterable


DEFAULT_HISTOGRAM_BUCKETS: tuple[float, ...] = (
    0.001,
    0.0025,
    0.005,
    0.01,
    0.025,
    0.05,
    0.1,
    0.25,
    0.5,
    1.0,
    2.5,
    5.0,
    10.0,
)


def _escape_label_value(value: Any) -> str:
    text = str(value)
    return text.replace("\\", "\\\\").replace("\n", "\\n").replace('"', '\\"')


def _format_labels(labels: dict[str, Any]) -> str:
    if not labels:
        return ""
    parts = [f'{key}="{_escape_label_value(value)}"' for key, value in sorted(labels.items())]
    return "{" + ",".join(parts) + "}"


@dataclass(slots=True)
class _Histogram:
    buckets: tuple[float, ...]
    counts: dict[float, int] = field(default_factory=dict)
    total: int = 0
    sum: float = 0.0

    def __post_init__(self) -> None:
        for bucket in self.buckets:
            self.counts.setdefault(bucket, 0)

    def observe(self, value: float) -> None:
        self.total += 1
        self.sum += value
        for bucket in self.buckets:
            if value <= bucket:
                self.counts[bucket] += 1


class MetricsCollector:
    """Thread-safe Prometheus-style metrics collector for the Python SDK."""

    def __init__(
        self,
        namespace: str = "loxa_sdk",
        *,
        buffer_capacity: int = 0,
        histogram_buckets: Iterable[float] = DEFAULT_HISTOGRAM_BUCKETS,
    ) -> None:
        self.namespace = namespace or "loxa_sdk"
        self._lock = Lock()
        self._counters: dict[str, int] = defaultdict(int)
        self._counter_vecs: dict[str, dict[str, int]] = defaultdict(lambda: defaultdict(int))
        self._gauges: dict[str, float] = {
            "buffer_size": 0.0,
            "buffer_capacity": float(max(0, buffer_capacity)),
        }
        self._emit_duration = _Histogram(tuple(sorted(set(float(b) for b in histogram_buckets))))

    def on_event_created(self) -> None:
        with self._lock:
            self._counters["events_created_total"] += 1

    def on_event_finished(self) -> None:
        with self._lock:
            self._counters["events_finished_total"] += 1

    def on_event_emitted(self, success: bool = True) -> None:
        with self._lock:
            key = "success" if success else "failure"
            self._counter_vecs["events_emitted_total"][key] += 1

    def on_event_dropped(self, reason: str) -> None:
        with self._lock:
            self._counter_vecs["events_dropped_total"][reason] += 1

    def on_retry(self, attempt: int) -> None:
        with self._lock:
            self._counter_vecs["retry_total"][str(max(1, int(attempt)))] += 1

    def on_backpressure(self) -> None:
        with self._lock:
            self._counters["backpressure_total"] += 1

    def observe_emit_duration(self, duration: float | timedelta) -> None:
        seconds = duration.total_seconds() if isinstance(duration, timedelta) else float(duration)
        seconds = max(0.0, seconds)
        with self._lock:
            self._emit_duration.observe(seconds)

    def set_buffer_size(self, size: int) -> None:
        with self._lock:
            self._gauges["buffer_size"] = float(max(0, size))

    def set_buffer_capacity(self, size: int) -> None:
        with self._lock:
            self._gauges["buffer_capacity"] = float(max(0, size))

    def snapshot(self) -> dict[str, Any]:
        with self._lock:
            return {
                "counters": dict(self._counters),
                "counter_vecs": {key: dict(value) for key, value in self._counter_vecs.items()},
                "gauges": dict(self._gauges),
                "emit_duration": {
                    "buckets": dict(self._emit_duration.counts),
                    "count": self._emit_duration.total,
                    "sum": self._emit_duration.sum,
                },
            }

    def render_prometheus(self) -> str:
        with self._lock:
            lines: list[str] = []
            self._append_counter(lines, "events_created_total", "Total number of events created via StartEvent")
            self._append_counter(lines, "events_finished_total", "Total number of events finished via Finish/FinishError")
            self._append_counter_vec(
                lines,
                "events_emitted_total",
                "Total number of events emitted to sink",
            )
            self._append_counter_vec(
                lines,
                "events_dropped_total",
                "Total number of events dropped",
            )
            self._append_counter_vec(
                lines,
                "retry_total",
                "Total number of retry attempts",
            )
            self._append_counter(lines, "backpressure_total", "Total number of backpressure events")
            self._append_gauge(lines, "buffer_size", "Current number of events in buffer")
            self._append_gauge(lines, "buffer_capacity", "Maximum buffer size")
            self._append_histogram(lines, "emit_duration_seconds", "Duration of Emit operations in seconds")
            return "\n".join(lines) + "\n"

    def _metric_name(self, name: str) -> str:
        return f"{self.namespace}_{name}"

    def _append_counter(self, lines: list[str], name: str, help_text: str) -> None:
        metric = self._metric_name(name)
        lines.append(f"# HELP {metric} {help_text}")
        lines.append(f"# TYPE {metric} counter")
        lines.append(f"{metric} {self._counters.get(name, 0)}")

    def _append_counter_vec(self, lines: list[str], name: str, help_text: str) -> None:
        metric = self._metric_name(name)
        lines.append(f"# HELP {metric} {help_text}")
        lines.append(f"# TYPE {metric} counter")
        for label, value in sorted(self._counter_vecs.get(name, {}).items()):
            lines.append(f"{metric}{_format_labels({'status' if name == 'events_emitted_total' else 'reason' if name == 'events_dropped_total' else 'attempt': label})} {value}")

    def _append_gauge(self, lines: list[str], name: str, help_text: str) -> None:
        metric = self._metric_name(name)
        lines.append(f"# HELP {metric} {help_text}")
        lines.append(f"# TYPE {metric} gauge")
        lines.append(f"{metric} {self._gauges.get(name, 0.0)}")

    def _append_histogram(self, lines: list[str], name: str, help_text: str) -> None:
        metric = self._metric_name(name)
        lines.append(f"# HELP {metric} {help_text}")
        lines.append(f"# TYPE {metric} histogram")
        cumulative = 0
        for bucket in self._emit_duration.buckets:
            cumulative = self._emit_duration.counts.get(bucket, cumulative)
            lines.append(f'{metric}_bucket{{le="{bucket:g}"}} {cumulative}')
        lines.append(f'{metric}_bucket{{le="+Inf"}} {self._emit_duration.total}')
        lines.append(f"{metric}_sum {self._emit_duration.sum}")
        lines.append(f"{metric}_count {self._emit_duration.total}")


def NewMetricsCollector(namespace: str = "loxa_sdk", *, buffer_capacity: int = 0) -> MetricsCollector:
    return MetricsCollector(namespace=namespace, buffer_capacity=buffer_capacity)


def RenderPrometheus(metrics: MetricsCollector) -> str:
    return metrics.render_prometheus()


MetricsSnapshot = dict[str, Any]
PrometheusMetrics = MetricsCollector


__all__ = [
    "DEFAULT_HISTOGRAM_BUCKETS",
    "MetricsCollector",
    "MetricsSnapshot",
    "NewMetricsCollector",
    "PrometheusMetrics",
    "RenderPrometheus",
]
