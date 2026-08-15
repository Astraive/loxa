"""Test that incident_id is properly set and emitted through the Python SDK."""

from __future__ import annotations

import json

import loza


def test_incident_id_in_params() -> None:
    """Verify incident_id set via Params appears in emitted output."""
    logger = loza.New(loza.Test("test-svc"))
    ctx = logger.start_event(
        loza.Params(
            event="test.incident",
            trace_id="trace-123",
            span_id="span-456",
            request_id="req-789",
            incident_id="incident-test-001",
        )
    )
    logger.finish(ctx, "success")
    payload = json.loads(logger.emit(ctx))
    assert payload.get("incident_id") == "incident-test-001", (
        f"expected incident_id=incident-test-001, got {payload.get('incident_id')}"
    )


def test_incident_id_in_params_enrich() -> None:
    """Verify incident_id can be set after start via Params.incident_id."""
    logger = loza.New(loza.Test("test-svc"))
    params = loza.Params(
        event="test.incident",
        incident_id="inc-enrich-test",
    )
    ctx = logger.start_event(params)
    logger.finish(ctx, "success")
    payload = json.loads(logger.emit(ctx))
    assert payload.get("incident_id") == "inc-enrich-test", (
        f"expected incident_id=inc-enrich-test, got {payload.get('incident_id')}"
    )


def test_incident_id_omitted_when_empty() -> None:
    """Verify incident_id is omitted from output when not set."""
    logger = loza.New(loza.Test("test-svc"))
    ctx = logger.start_event(loza.Params(event="test.no-incident"))
    logger.finish(ctx, "success")
    payload = json.loads(logger.emit(ctx))
    assert "incident_id" not in payload, "incident_id should not be present in output when not set"


def test_incident_id_preserved_through_all_policies() -> None:
    """Verify incident_id survives all duplicate field policies."""
    for policy in [loza.CanonicalWins, loza.UserWins, loza.KeepBoth]:
        logger = loza.New(loza.Test("test-svc").with_duplicate_policy(policy))
        ctx = logger.start_event(
            loza.Params(
                event="test.incident-policy",
                incident_id="inc-policy-test",
            )
        )
        logger.finish(ctx, "success")
        payload = json.loads(logger.emit(ctx))
        assert payload.get("incident_id") == "inc-policy-test", (
            f"expected incident_id=inc-policy-test with {policy.__name__}, got {payload.get('incident_id')}"
        )
