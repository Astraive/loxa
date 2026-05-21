#!/usr/bin/env python3
"""Queue adapters for ingestion.

LocalQueue is a file-backed queue for local development/testing.
KafkaAdapter provides real Kafka integration via confluent-kafka.
"""
from __future__ import annotations

import json
import os
import time
from pathlib import Path
from typing import Any, Iterable

from .kafka_adapter import KafkaAdapter, KafkaConfig, KafkaRecord, RetryPolicy


class LocalQueue:
    """Simple file-backed queue. Each message is one JSON file.

    Methods:
    - produce(obj): append message file
    - consume(batch_size=10): yield up to batch_size messages (path, payload)
    - ack(path): remove processed file
    """

    def __init__(self, path: str | Path, dlq_path: str | Path | None = None):
        self.path = Path(path)
        self.path.mkdir(parents=True, exist_ok=True)
        self.dlq_path = Path(dlq_path) if dlq_path is not None else self.path / "_dlq"
        self.dlq_path.mkdir(parents=True, exist_ok=True)

    def _write_message(self, base: Path, payload: dict[str, Any], prefix: str = "msg") -> str:
        ts = int(time.time() * 1000)
        suffix = int(time.time() * 1000000) % 1000000
        name = f"{prefix}-{ts}-{os.getpid()}-{suffix}.json"
        target = base / name
        target.write_text(json.dumps(payload, separators=(",", ":"), ensure_ascii=False), encoding="utf-8")
        return str(target)

    def produce(self, obj: dict) -> str:
        return self._write_message(self.path, obj, prefix="msg")

    def consume(self, batch_size: int = 10) -> Iterable[tuple[Path, dict]]:
        files = sorted(self.path.glob("*.json"))[:batch_size]
        for f in files:
            try:
                payload = json.loads(f.read_text(encoding="utf-8"))
            except Exception:
                # Skip invalid files but don't crash
                continue
            yield f, payload

    def ack(self, file_path: Path) -> None:
        try:
            file_path.unlink()
        except Exception:
            pass

    def send_to_dlq(self, payload: dict[str, Any], *, reason: str, attempts: int, source: Any = None) -> str:
        body = {
            "payload": payload,
            "reason": reason,
            "attempts": attempts,
            "source": str(source) if source is not None else None,
            "failed_at_epoch_ms": int(time.time() * 1000),
        }
        return self._write_message(self.dlq_path, body, prefix="dlq")


if __name__ == "__main__":
    q = LocalQueue("./tmp_ingest_queue")
    q.produce(
        {
            "event_id": "test-1",
            "schema_version": "v1",
            "event_version": "v1",
            "timestamp": "2026-05-18T12:30:45Z",
            "service": "local",
            "event": "test",
        }
    )
    print("queue files:", list(q.path.glob("*.json")))
