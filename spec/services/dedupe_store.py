#!/usr/bin/env python3
"""Dedupe store prototype: Redis-backed with local fallback.

Provides a tiny interface for idempotency windows: mark_seen(event_id, ttl_seconds) and
is_seen(event_id). Redis is preferred; if redis-py is not installed or REDIS_URL is not set,
falls back to a local file-backed store using shelve.
"""
from __future__ import annotations

import os
import time
from pathlib import Path
from typing import Optional


class LocalDedupeStore:
    def __init__(self, path: str | Path = "./.dedupe_store") -> None:
        import shelve

        self.path = Path(path)
        self.path.parent.mkdir(parents=True, exist_ok=True)
        self._shelf_path = str(self.path)

    def mark_seen(self, key: str, ttl: int) -> bool:
        import shelve

        with shelve.open(self._shelf_path) as db:
            now = int(time.time())
            expiry = now + ttl
            if key in db:
                val = db[key]
                if val >= now:
                    return False
            db[key] = expiry
            return True

    def is_seen(self, key: str) -> bool:
        import shelve

        with shelve.open(self._shelf_path) as db:
            now = int(time.time())
            if key in db:
                expiry = db[key]
                if expiry >= now:
                    return True
                # expired
                del db[key]
            return False


class RedisDedupeStore:
    def __init__(self, redis_client) -> None:
        self.redis = redis_client

    def mark_seen(self, key: str, ttl: int) -> bool:
        # SETNX with expiry: set if not exists, return True if set
        was_set = self.redis.set(key, "1", nx=True, ex=ttl)
        return bool(was_set)

    def is_seen(self, key: str) -> bool:
        return bool(self.redis.get(key))


def build_store() -> tuple[object, str]:
    """Return (store, backend) where backend is 'redis' or 'local'."""
    redis_url = os.environ.get("REDIS_URL")
    if redis_url:
        try:
            import redis

            client = redis.from_url(redis_url, decode_responses=True)
            # simple ping
            client.ping()
            return RedisDedupeStore(client), "redis"
        except Exception:
            pass
    # fallback
    return LocalDedupeStore(), "local"


if __name__ == "__main__":
    store, backend = build_store()
    print("Using backend:", backend)
    test_key = "evt-test-123"
    print("first mark_seen ->", store.mark_seen(test_key, 10))
    print("second mark_seen ->", store.mark_seen(test_key, 10))
    print("is_seen ->", store.is_seen(test_key))
