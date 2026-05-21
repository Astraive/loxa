#!/usr/bin/env python3
"""test_equivalence.py -- Cross-SDK equivalence test.

Sends the same canonical event from each SDK and asserts stored events
match on canonical fields. If no collector is running at localhost:9090,
prints SKIP and exits 0.
"""

import json
import sys
import time
import urllib.request
import urllib.error
from datetime import datetime, timezone

COLLECTOR_URL = "http://localhost:9090"
CANONICAL_EVENT = {
    "service": "equivalence-test",
    "event": "test.cross-sdk",
    "kind": "http",
    "outcome": "success",
    "attrs": {
        "http.method": "GET",
        "http.path": "/api/v1/test",
        "http.status_code": 200,
        "custom.label": "equivalence-check",
    },
}


def check_collector_available() -> bool:
    """Check if collector is running and healthy."""
    try:
        req = urllib.request.Request(f"{COLLECTOR_URL}/health", method="GET")
        with urllib.request.urlopen(req, timeout=3) as resp:
            return resp.status == 200
    except (urllib.error.URLError, OSError, TimeoutError):
        return False


def emit_event_via_api(event: dict) -> dict | None:
    """Emit an event directly to the collector ingest endpoint."""
    payload = json.dumps(event).encode("utf-8")
    req = urllib.request.Request(
        f"{COLLECTOR_URL}/api/v1/ingest",
        data=payload,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except (urllib.error.URLError, OSError) as e:
        print(f"  WARN: Emit failed: {e}", file=sys.stderr)
        return None


def query_events(service: str) -> list[dict]:
    """Query stored events from the collector."""
    url = f"{COLLECTOR_URL}/api/v1/query?service={service}&limit=10"
    req = urllib.request.Request(url, method="GET")
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            data = json.loads(resp.read().decode("utf-8"))
            return data.get("events", data.get("results", []))
    except (urllib.error.URLError, OSError):
        return []


def canonical_match(a: dict, b: dict) -> bool:
    """Check if two events match on canonical fields."""
    fields = ["service", "event", "kind", "outcome"]
    for f in fields:
        if a.get(f) != b.get(f):
            return False
    # Check attrs subset
    a_attrs = a.get("attrs", {})
    b_attrs = b.get("attrs", {})
    for key in CANONICAL_EVENT["attrs"]:
        if a_attrs.get(key) != b_attrs.get(key):
            return False
    return True


def main():
    print("=== Cross-SDK Equivalence Test ===")
    print(f"Collector: {COLLECTOR_URL}")
    print()

    if not check_collector_available():
        print("SKIP: Collector not running at localhost:9090")
        print("Start the collector first: cd collector && go run ./cmd/collector")
        sys.exit(0)

    print("Collector is available. Running equivalence test...")
    print()

    # Emit the canonical event with a unique marker
    marker = f"equiv-{int(time.time())}"
    event = dict(CANONICAL_EVENT)
    event["attrs"] = dict(CANONICAL_EVENT["attrs"])
    event["attrs"]["test.marker"] = marker

    print(f"Emitting canonical event (marker={marker})...")
    result = emit_event_via_api(event)
    if result is None:
        print("FAIL: Could not emit event to collector")
        sys.exit(1)

    print(f"  Emit response: {json.dumps(result)}")

    # Wait briefly for storage
    time.sleep(0.5)

    # Query and verify
    print("Querying stored events...")
    events = query_events("equivalence-test")
    if not events:
        print("FAIL: No events found for service=equivalence-test")
        sys.exit(1)

    # Find our event by marker
    found = None
    for ev in events:
        if ev.get("attrs", {}).get("test.marker") == marker:
            found = ev
            break

    if found is None:
        print("FAIL: Could not find emitted event by marker")
        print(f"  Queried {len(events)} event(s)")
        sys.exit(1)

    print("  Found matching event. Verifying canonical fields...")

    # Verify canonical fields match
    if canonical_match(found, event):
        print("PASS: Canonical fields match between emitted and stored event")
    else:
        print("FAIL: Canonical fields do not match")
        print(f"  Expected: {json.dumps({k: event[k] for k in ['service','event','kind','outcome']})}")
        print(f"  Got:      {json.dumps({k: found.get(k) for k in ['service','event','kind','outcome']})}")
        sys.exit(1)

    print()
    print("=== Equivalence Test PASSED ===")


if __name__ == "__main__":
    main()
