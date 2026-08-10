from __future__ import annotations

import loza


def test_testing_and_conformance_helpers():
    logger, sink = loza.TestLogger()
    logger.info("hello")
    events = loza.DecodeEvents(sink)

    found = loza.expect_event(events, message="hello")
    assert found is not None
    loza.expect_attr(found, "message", "hello")

    snap = loza.snapshot_event(found)
    assert "event_id" not in snap
    assert "timestamp" not in snap
    assert isinstance(loza.mock_sink(), loza.MemorySink)

    loza.set_id_generator(lambda: "evt_fixed")
    logger2, sink2 = loza.TestLogger()
    logger2.info("id-fixed")
    fixed = loza.DecodeEvents(sink2)[0]
    assert fixed.get("event_id") == "evt_fixed"
    loza.reset_for_test()
