#!/usr/bin/env python3
"""Collector example: receives events, enforces dedupe via dedupe_store, and enqueues to LocalQueue.

This is a lightweight prototype demonstrating how collector can be stateless, use Redis for
idempotency, and push events into a durable queue for downstream processing.
"""
from __future__ import annotations

import json
from pathlib import Path
from typing import Any

import sys
from pathlib import Path
# Make services/ importable when running as a standalone script
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from services.queue_prototype import LocalQueue
from services.dedupe_store import build_store


def handle_event(event: dict[str, Any], queue: LocalQueue, dedupe_store) -> dict:
    event_id = event.get("event_id")
    if not event_id:
        return {"status": "invalid", "reason": "missing_event_id"}
    # TTL for dedupe window in seconds
    ttl = 60 * 60
    allowed = dedupe_store.mark_seen(event_id, ttl)
    if not allowed:
        return {"status": "duplicate", "event_id": event_id}
    # enqueue
    queue.produce(event)
    return {"status": "accepted", "event_id": event_id}


if __name__ == "__main__":
    queue = LocalQueue("./tmp_ingest_queue")
    store, backend = build_store()
    sample = {"event_id": "evt-1001", "schema_version": "v1", "event_version": "v1", "timestamp": "2026-05-18T12:30:45Z", "service": "example", "event": "demo"}
    print("backend:", backend)
    print(handle_event(sample, queue, store))
    print(handle_event(sample, queue, store))
    print("queue files:", list(queue.path.glob('*.json')))
