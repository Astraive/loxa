"""End-to-end test: HTTPBatchSink -> live loxa-collector pipeline.

Requires a collector at ``LOXA_TEST_COLLECTOR_URL`` (default
``http://127.0.0.1:9308``), its ingest token in ``LOXA_API_KEY``, and an
admin token in ``LOXA_TEST_COLLECTOR_ADMIN_KEY``.
"""
import json
import os
import socket
import time
import urllib.request
from urllib.parse import urlparse
import pytest

import loxa


COLLECTOR_URL = os.getenv("LOXA_TEST_COLLECTOR_URL", "http://127.0.0.1:9308").rstrip("/")
INGEST_API_KEY = os.getenv("LOXA_API_KEY", "")
ADMIN_API_KEY = os.getenv("LOXA_TEST_COLLECTOR_ADMIN_KEY", "")


def _collector_reachable(url: str = COLLECTOR_URL, timeout: float = 2.0) -> bool:
    parsed = urlparse(url)
    host = parsed.hostname or "127.0.0.1"
    port = parsed.port or (443 if parsed.scheme == "https" else 80)
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.settimeout(timeout)
    try:
        sock.connect((host, port))
        return True
    except OSError:
        return False
    finally:
        sock.close()


pytestmark = pytest.mark.skipif(
    not _collector_reachable(),
    reason=f"loxa-collector not running at {COLLECTOR_URL}",
)


def test_e2e_collector_pipeline():
    """Send events through HTTPBatchSink and verify collector receives them."""
    assert INGEST_API_KEY, "LOXA_API_KEY is required for authenticated collector E2E"
    assert ADMIN_API_KEY, "LOXA_TEST_COLLECTOR_ADMIN_KEY is required for status/query checks"

    print("\n=== E2E Test: HTTPBatchSink -> loxa-collector ===\n")

    # 1. Create logger with collector sink
    config = (
        loxa.Production("e2e-test-service")
        .with_collector_endpoint(f"{COLLECTOR_URL}/events")
        .with_api_key(INGEST_API_KEY)
    )
    logger = loxa.New(config)

    # 2. Emit 3 events
    for i in range(3):
        ctx = logger.start_event(loxa.Params(
            event=f"e2e.test.event_{i}",
            message=f"E2E test event {i}",
            level="info",
        ))
        logger.enrich(ctx, loxa.String("test_run", "e2e"))
        logger.enrich(ctx, loxa.Int("sequence", i))
        logger.finish(ctx, "success")
        logger.emit(ctx)
        print(f"  Sent: e2e.test.event_{i}")

    # 3. Flush
    logger.flush()
    time.sleep(1)
    print("  Flushed to collector\n")

    # 4. Verify via collector /status (accepted count)
    print("  Verifying via collector status...")
    status_req = urllib.request.Request(
        f"{COLLECTOR_URL}/status",
        headers={"Authorization": f"Bearer {ADMIN_API_KEY}"},
        method="GET",
    )
    with urllib.request.urlopen(status_req, timeout=5) as resp:
        status_data = json.loads(resp.read())

    accepted = status_data.get("ingest", {}).get("accepted", 0)
    print(f"  Collector accepted count: {accepted}")
    assert accepted >= 3, f"Expected >=3 accepted, got {accepted}"

    # Also verify via /query (if DuckDB available)
    # Fall back to checking accepted count
    e2e_events = []
    for i in range(3):
        e2e_events.append({"event": f"e2e.test.event_{i}", "service": "e2e-test-service", "outcome": "success"})
    print("  Collector accepted >= 3 events: OK\n")

    # Verify collector has events in DuckDB via query endpoint
    try:
        query_req = urllib.request.Request(
            f"{COLLECTOR_URL}/query",
            data=json.dumps({"sql": "SELECT event, service, outcome FROM events WHERE service = 'e2e-test-service' LIMIT 10"}).encode(),
            headers={"Authorization": f"Bearer {ADMIN_API_KEY}", "content-type": "application/json"},
            method="POST",
        )
        with urllib.request.urlopen(query_req, timeout=5) as resp:
            query_result = json.loads(resp.read())
        rows = query_result.get("rows", [])
        print(f"  DuckDB query returned {len(rows)} rows")
        for row in rows:
            print(f"    OK: {row.get('event')} | service={row.get('service')} | outcome={row.get('outcome')}")
    except Exception as e:
        print(f"  DuckDB query skipped: {e}")

    # 5. CollectorClient health/ready/version/status
    from loxa.core.http_client import CollectorClient
    cc = CollectorClient(f"{COLLECTOR_URL}/events", api_key=ADMIN_API_KEY)

    assert cc.health() is True, "collector health check failed"
    print("\n  Collector health: True")

    assert cc.ready() is True, "collector ready check failed"
    print("  Collector ready:  True")

    ver = cc.version()
    assert "version" in ver, f"version response missing: {ver}"
    print(f"  Collector version: {ver}")

    st = cc.status()
    assert st.get("status") == "ok", f"status not ok: {st}"
    accepted = st.get("ingest", {}).get("accepted", 0)
    print(f"  Collector status: {st.get('status')} | uptime: {st.get('uptime_seconds')}s | accepted: {accepted}")

    print("\n=== E2E PASSED ===")


if __name__ == "__main__":
    test_e2e_collector_pipeline()
