#!/usr/bin/env python3
"""Stager worker: consumes queue messages and stages partitioned event files safely."""
from __future__ import annotations

import argparse
import gzip
import json
import logging
import os
import random
import re
import time
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from .kafka_adapter import KafkaAdapter, KafkaConfig, RetryPolicy, compute_backoff_seconds
from .queue_prototype import LocalQueue

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")


def _env_int(name: str, default: int) -> int:
    value = os.getenv(name)
    if value is None:
        return default
    try:
        return int(value)
    except ValueError:
        return default


def _env_float(name: str, default: float) -> float:
    value = os.getenv(name)
    if value is None:
        return default
    try:
        return float(value)
    except ValueError:
        return default


def _env_bool(name: str, default: bool) -> bool:
    value = os.getenv(name)
    if value is None:
        return default
    return value.strip().lower() in {"1", "true", "yes", "on"}


def _env_str(name: str, default: str) -> str:
    value = os.getenv(name)
    if value is None:
        return default
    value = value.strip()
    return value or default


def _partition_path(base: Path, ts: str) -> Path:
    # ts expected RFC3339 or ISO date-like; fallback to now
    try:
        dt = datetime.fromisoformat(ts.replace("Z", "+00:00"))
    except Exception:
        dt = datetime.now(timezone.utc)
    return base / f"year={dt.year}" / f"month={dt.month:02d}" / f"day={dt.day:02d}"


def _safe_name(value: str) -> str:
    cleaned = re.sub(r"[^A-Za-z0-9._-]+", "-", value.strip())
    return cleaned[:200] if cleaned else "event"


def _source_suffix(source: Any) -> str:
    if isinstance(source, Path):
        return source.stem
    partition = getattr(source, "partition", None)
    offset = getattr(source, "offset", None)
    if partition is not None and offset is not None:
        return f"p{partition}-o{offset}"
    return _safe_name(str(source))


def _build_target(partition_dir: Path, payload: dict[str, Any], source: Any, compress: bool) -> Path:
    ext = ".json.gz" if compress else ".json"
    event_id = payload.get("event_id")
    stem = _safe_name(str(event_id)) if event_id else f"noid-{_source_suffix(source)}"
    candidate = partition_dir / f"{stem}{ext}"
    if not candidate.exists():
        return candidate
    i = 1
    while True:
        retry_candidate = partition_dir / f"{stem}-{i}{ext}"
        if not retry_candidate.exists():
            return retry_candidate
        i += 1


def _write_payload_atomically(target: Path, payload: dict[str, Any], compress: bool) -> None:
    target.parent.mkdir(parents=True, exist_ok=True)
    temp_file = target.with_name(f".{target.name}.{os.getpid()}.{int(time.time() * 1000000)}.tmp")
    try:
        if compress:
            with gzip.open(temp_file, "wt", encoding="utf-8") as fh:
                json.dump(payload, fh, separators=(",", ":"), ensure_ascii=False)
        else:
            temp_file.write_text(json.dumps(payload, separators=(",", ":"), ensure_ascii=False), encoding="utf-8")
        os.replace(temp_file, target)
    finally:
        if temp_file.exists():
            try:
                temp_file.unlink()
            except Exception:
                pass


def _stage_payload(staging_dir: Path, payload: dict[str, Any], source: Any, compress: bool) -> Path:
    ts = payload.get("timestamp") or datetime.now(timezone.utc).isoformat()
    partition_dir = _partition_path(staging_dir, ts)
    target = _build_target(partition_dir, payload, source, compress)
    _write_payload_atomically(target, payload, compress)
    return target


def stage_events(
    queue: Any,
    staging_dir: Path,
    batch_size: int = 10,
    compress: bool = True,
    retry_policy: RetryPolicy | None = None,
    *,
    sleep_fn=time.sleep,
    random_fn=random.random,
) -> int:
    staged = 0
    policy = retry_policy or RetryPolicy()
    staging_dir.mkdir(parents=True, exist_ok=True)
    for source, payload in queue.consume(batch_size=batch_size):
        attempts = 0
        try:
            while attempts < policy.max_attempts:
                attempts += 1
                try:
                    _stage_payload(staging_dir, payload, source, compress)
                    queue.ack(source)
                    staged += 1
                    break
                except Exception as exc:
                    logging.warning(
                        "stage attempt failed source=%s attempt=%d/%d error=%s",
                        _source_suffix(source),
                        attempts,
                        policy.max_attempts,
                        exc,
                    )
                    if attempts >= policy.max_attempts:
                        if hasattr(queue, "send_to_dlq"):
                            logging.error(
                                "stage attempts exhausted source=%s attempts=%d error=%s",
                                _source_suffix(source),
                                attempts,
                                exc,
                            )
                            queue.send_to_dlq(
                                payload,
                                reason=f"stage-failed: {exc}",
                                attempts=attempts,
                                source=source,
                            )
                            queue.ack(source)
                        break
                    sleep_fn(compute_backoff_seconds(attempts, policy, random_fn))
        except Exception:
            continue
    return staged


def _write_heartbeat(path: Path | None) -> None:
    if path is None:
        return
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(str(int(time.time())), encoding="utf-8")


def build_queue(
    backend: str,
    *,
    queue_dir: str,
    kafka_brokers: str,
    kafka_topic: str,
    kafka_group_id: str,
    kafka_dlq_topic: str,
    kafka_auto_offset_reset: str,
    kafka_poll_timeout_seconds: float,
    kafka_flush_timeout_seconds: float,
    retry_policy: RetryPolicy,
) -> Any:
    backend = backend.strip().lower()
    if backend == "local":
        return LocalQueue(queue_dir)
    if backend == "kafka":
        missing = [
            name
            for name, value in (
                ("KAFKA_BOOTSTRAP_SERVERS", kafka_brokers),
                ("KAFKA_TOPIC", kafka_topic),
                ("KAFKA_GROUP_ID", kafka_group_id),
                ("KAFKA_DLQ_TOPIC", kafka_dlq_topic),
            )
            if not value.strip()
        ]
        if missing:
            raise ValueError(f"missing required Kafka configuration: {', '.join(missing)}")
        return KafkaAdapter(
            KafkaConfig(
                brokers=kafka_brokers,
                topic=kafka_topic,
                group_id=kafka_group_id,
                dlq_topic=kafka_dlq_topic,
                auto_offset_reset=kafka_auto_offset_reset,
                consumer_poll_timeout_seconds=kafka_poll_timeout_seconds,
                producer_flush_timeout_seconds=kafka_flush_timeout_seconds,
            ),
            retry_policy=retry_policy,
        )
    raise ValueError(f"unsupported stager queue backend: {backend}")


def run_worker(
    backend: str,
    queue_dir: str,
    staging_dir: Path,
    batch_size: int,
    compress: bool,
    retry_policy: RetryPolicy,
    *,
    kafka_brokers: str = "",
    kafka_topic: str = "",
    kafka_group_id: str = "",
    kafka_dlq_topic: str = "",
    kafka_auto_offset_reset: str = "earliest",
    kafka_poll_timeout_seconds: float = 1.0,
    kafka_flush_timeout_seconds: float = 10.0,
    loop: bool = False,
    poll_interval_seconds: float = 2.0,
    heartbeat_file: Path | None = None,
    sleep_fn=time.sleep,
) -> int:
    queue = build_queue(
        backend,
        queue_dir=queue_dir,
        kafka_brokers=kafka_brokers,
        kafka_topic=kafka_topic,
        kafka_group_id=kafka_group_id,
        kafka_dlq_topic=kafka_dlq_topic,
        kafka_auto_offset_reset=kafka_auto_offset_reset,
        kafka_poll_timeout_seconds=kafka_poll_timeout_seconds,
        kafka_flush_timeout_seconds=kafka_flush_timeout_seconds,
        retry_policy=retry_policy,
    )
    total_staged = 0
    logging.info(
        "stager starting backend=%s batch_size=%d compress=%s staging_dir=%s",
        backend,
        batch_size,
        compress,
        staging_dir,
    )
    try:
        while True:
            staged = stage_events(queue, staging_dir, batch_size=batch_size, compress=compress, retry_policy=retry_policy)
            total_staged += staged
            _write_heartbeat(heartbeat_file)
            if staged > 0:
                logging.info("staged=%d total=%d backend=%s", staged, total_staged, backend)
            if not loop:
                return total_staged
            sleep_fn(poll_interval_seconds)
    finally:
        close_fn = getattr(queue, "close", None)
        if callable(close_fn):
            close_fn()


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--queue-backend",
        default=_env_str("STAGER_QUEUE_BACKEND", "local"),
        choices=("local", "kafka"),
    )
    parser.add_argument("--queue-dir", default=os.getenv("STAGER_QUEUE_DIR", "./tmp_ingest_queue"))
    parser.add_argument("--staging-dir", default=os.getenv("STAGER_STAGING_DIR", "./tmp_staging"))
    parser.add_argument("--batch-size", type=int, default=_env_int("STAGER_BATCH_SIZE", 100))
    parser.add_argument("--loop", action="store_true", default=_env_bool("STAGER_LOOP", False))
    parser.add_argument(
        "--poll-interval-seconds",
        type=float,
        default=_env_float("STAGER_POLL_INTERVAL_SECONDS", 2.0),
    )
    parser.add_argument("--heartbeat-file", default=os.getenv("STAGER_HEARTBEAT_FILE"))
    parser.add_argument("--max-attempts", type=int, default=_env_int("STAGER_MAX_ATTEMPTS", 5))
    parser.add_argument(
        "--base-delay-seconds",
        type=float,
        default=_env_float("STAGER_BASE_DELAY_SECONDS", 0.2),
    )
    parser.add_argument(
        "--max-delay-seconds",
        type=float,
        default=_env_float("STAGER_MAX_DELAY_SECONDS", 5.0),
    )
    parser.add_argument("--jitter-ratio", type=float, default=_env_float("STAGER_JITTER_RATIO", 0.2))
    parser.add_argument("--kafka-brokers", default=_env_str("KAFKA_BOOTSTRAP_SERVERS", ""))
    parser.add_argument("--kafka-topic", default=_env_str("KAFKA_TOPIC", ""))
    parser.add_argument("--kafka-group-id", default=_env_str("KAFKA_GROUP_ID", ""))
    parser.add_argument("--kafka-dlq-topic", default=_env_str("KAFKA_DLQ_TOPIC", ""))
    parser.add_argument(
        "--kafka-auto-offset-reset",
        default=_env_str("KAFKA_AUTO_OFFSET_RESET", "earliest"),
    )
    parser.add_argument(
        "--kafka-poll-timeout-seconds",
        type=float,
        default=_env_float("KAFKA_POLL_TIMEOUT_SECONDS", 1.0),
    )
    parser.add_argument(
        "--kafka-flush-timeout-seconds",
        type=float,
        default=_env_float("KAFKA_FLUSH_TIMEOUT_SECONDS", 10.0),
    )
    compress_default = _env_bool("STAGER_COMPRESS", True)
    compress_group = parser.add_mutually_exclusive_group()
    compress_group.add_argument("--compress", dest="compress", action="store_true")
    compress_group.add_argument("--no-compress", dest="compress", action="store_false")
    parser.set_defaults(compress=compress_default)

    args = parser.parse_args()
    policy = RetryPolicy(
        max_attempts=args.max_attempts,
        base_delay_seconds=args.base_delay_seconds,
        max_delay_seconds=args.max_delay_seconds,
        jitter_ratio=args.jitter_ratio,
    )
    heartbeat_file = Path(args.heartbeat_file) if args.heartbeat_file else None
    try:
        staged = run_worker(
            backend=args.queue_backend,
            queue_dir=args.queue_dir,
            staging_dir=Path(args.staging_dir),
            batch_size=args.batch_size,
            compress=args.compress,
            retry_policy=policy,
            kafka_brokers=args.kafka_brokers,
            kafka_topic=args.kafka_topic,
            kafka_group_id=args.kafka_group_id,
            kafka_dlq_topic=args.kafka_dlq_topic,
            kafka_auto_offset_reset=args.kafka_auto_offset_reset,
            kafka_poll_timeout_seconds=args.kafka_poll_timeout_seconds,
            kafka_flush_timeout_seconds=args.kafka_flush_timeout_seconds,
            loop=args.loop,
            poll_interval_seconds=args.poll_interval_seconds,
            heartbeat_file=heartbeat_file,
        )
        print(f"staged={staged}")
    except KeyboardInterrupt:
        logging.info("stager interrupted, exiting")
