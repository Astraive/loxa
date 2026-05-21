#!/usr/bin/env python3
"""Kafka queue adapter backed by confluent-kafka."""
from __future__ import annotations

import json
import random
import time
from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import Any, Callable, Iterable, Optional


@dataclass(frozen=True)
class RetryPolicy:
    max_attempts: int = 3
    base_delay_seconds: float = 0.25
    max_delay_seconds: float = 10.0
    jitter_ratio: float = 0.2


def compute_backoff_seconds(
    attempt: int,
    retry_policy: RetryPolicy,
    random_fn: Callable[[], float] = random.random,
) -> float:
    base_delay = retry_policy.base_delay_seconds * (2 ** max(0, attempt - 1))
    capped_delay = min(retry_policy.max_delay_seconds, base_delay)
    jitter = capped_delay * retry_policy.jitter_ratio * max(0.0, random_fn())
    return min(retry_policy.max_delay_seconds, capped_delay + jitter)


@dataclass(frozen=True)
class KafkaConfig:
    brokers: str
    topic: str
    group_id: str
    dlq_topic: str
    auto_offset_reset: str = "earliest"
    consumer_poll_timeout_seconds: float = 1.0
    producer_flush_timeout_seconds: float = 10.0
    enable_auto_commit: bool = False
    extra_consumer_config: dict[str, Any] = field(default_factory=dict)
    extra_producer_config: dict[str, Any] = field(default_factory=dict)


@dataclass(frozen=True)
class KafkaRecord:
    topic: str
    partition: int
    offset: int
    key: bytes | None
    raw_value: bytes | None
    _message: Any

    @classmethod
    def from_message(cls, message: Any) -> "KafkaRecord":
        return cls(
            topic=message.topic(),
            partition=message.partition(),
            offset=message.offset(),
            key=message.key(),
            raw_value=message.value(),
            _message=message,
        )


class KafkaAdapter:
    def __init__(
        self,
        config: KafkaConfig,
        retry_policy: RetryPolicy | None = None,
        *,
        producer_cls: Optional[Callable[..., Any]] = None,
        consumer_cls: Optional[Callable[..., Any]] = None,
        kafka_error_cls: Any = None,
        sleep_fn: Callable[[float], None] = time.sleep,
        random_fn: Callable[[], float] = random.random,
    ) -> None:
        self.config = config
        self.retry_policy = retry_policy or RetryPolicy()
        self.sleep_fn = sleep_fn
        self.random_fn = random_fn

        if producer_cls is None or consumer_cls is None:
            try:
                from confluent_kafka import Consumer, KafkaError, Producer
            except ImportError as exc:
                raise RuntimeError(
                    "confluent-kafka is required for KafkaAdapter. "
                    "Install with: pip install confluent-kafka"
                ) from exc
            producer_cls = producer_cls or Producer
            consumer_cls = consumer_cls or Consumer
            kafka_error_cls = kafka_error_cls or KafkaError

        self.kafka_error_cls = kafka_error_cls
        producer_config = {"bootstrap.servers": self.config.brokers}
        producer_config.update(self.config.extra_producer_config)
        consumer_config = {
            "bootstrap.servers": self.config.brokers,
            "group.id": self.config.group_id,
            "auto.offset.reset": self.config.auto_offset_reset,
            "enable.auto.commit": self.config.enable_auto_commit,
        }
        consumer_config.update(self.config.extra_consumer_config)

        self.producer = producer_cls(producer_config)
        self.consumer = consumer_cls(consumer_config)
        self.consumer.subscribe([self.config.topic])

    def produce(
        self,
        obj: dict[str, Any],
        *,
        topic: str | None = None,
        key: bytes | str | None = None,
        headers: dict[str, str] | None = None,
    ) -> None:
        payload = json.dumps(obj, separators=(",", ":"), ensure_ascii=False).encode("utf-8")
        key_bytes = key.encode("utf-8") if isinstance(key, str) else key
        self._produce_with_retry(topic or self.config.topic, payload, key=key_bytes, headers=headers)

    def send_to_dlq(
        self,
        payload: dict[str, Any],
        *,
        reason: str,
        attempts: int,
        source: Any = None,
    ) -> None:
        source_meta = None
        if isinstance(source, KafkaRecord):
            source_meta = {
                "topic": source.topic,
                "partition": source.partition,
                "offset": source.offset,
            }
        envelope = {
            "payload": payload,
            "reason": reason,
            "attempts": attempts,
            "failed_at": datetime.now(timezone.utc).isoformat(),
            "source": source_meta,
        }
        headers = {"x-loxa-dlq-reason": reason, "x-loxa-dlq-attempts": str(attempts)}
        self.produce(envelope, topic=self.config.dlq_topic, headers=headers)

    def consume(self, batch_size: int = 10, timeout: float | None = None) -> Iterable[tuple[KafkaRecord, dict]]:
        poll_timeout = self.config.consumer_poll_timeout_seconds if timeout is None else timeout
        yielded = 0
        while yielded < batch_size:
            message = self.consumer.poll(poll_timeout)
            if message is None:
                break
            err = message.error()
            if err:
                if self._is_partition_eof(err):
                    continue
                raise RuntimeError(f"Kafka consume error: {err}")

            record = KafkaRecord.from_message(message)
            raw = record.raw_value or b""
            try:
                payload = json.loads(raw.decode("utf-8"))
            except Exception as exc:
                self.send_to_dlq(
                    {"raw": raw.decode("utf-8", errors="replace")},
                    reason=f"invalid-json: {exc}",
                    attempts=1,
                    source=record,
                )
                self.ack(record)
                continue

            yielded += 1
            yield record, payload

    def ack(self, source: KafkaRecord) -> None:
        self.consumer.commit(message=source._message, asynchronous=False)

    def close(self) -> None:
        self.producer.flush(self.config.producer_flush_timeout_seconds)
        self.consumer.close()

    def _is_partition_eof(self, err: Any) -> bool:
        if self.kafka_error_cls is None:
            return False
        code_fn = getattr(err, "code", None)
        if not callable(code_fn):
            return False
        partition_eof = getattr(self.kafka_error_cls, "_PARTITION_EOF", None)
        if partition_eof is None:
            return False
        return code_fn() == partition_eof

    def _produce_with_retry(
        self,
        topic: str,
        payload: bytes,
        *,
        key: bytes | None = None,
        headers: dict[str, str] | None = None,
    ) -> None:
        last_error: Exception | None = None
        for attempt in range(1, self.retry_policy.max_attempts + 1):
            try:
                self._produce_once(topic, payload, key=key, headers=headers)
                return
            except Exception as exc:
                last_error = exc
                if attempt >= self.retry_policy.max_attempts:
                    break
                self.sleep_fn(compute_backoff_seconds(attempt, self.retry_policy, self.random_fn))
        raise RuntimeError(
            f"Kafka produce failed after {self.retry_policy.max_attempts} attempts to topic '{topic}'"
        ) from last_error

    def _produce_once(
        self,
        topic: str,
        payload: bytes,
        *,
        key: bytes | None = None,
        headers: dict[str, str] | None = None,
    ) -> None:
        delivery_error: dict[str, Any] = {"value": None}

        def _delivery_report(err: Any, _msg: Any) -> None:
            delivery_error["value"] = err

        self.producer.produce(
            topic,
            value=payload,
            key=key,
            headers=headers,
            on_delivery=_delivery_report,
        )
        self.producer.poll(0)
        pending = self.producer.flush(self.config.producer_flush_timeout_seconds)
        if pending:
            raise RuntimeError(f"Kafka flush timed out with {pending} pending messages")
        if delivery_error["value"] is not None:
            raise RuntimeError(f"Kafka delivery error: {delivery_error['value']}")
