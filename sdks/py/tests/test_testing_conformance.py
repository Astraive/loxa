from __future__ import annotations

import loxa


def test_testing_and_conformance_helpers():
    logger, sink = loxa.TestLogger()
    logger.info("hello")
    events = loxa.DecodeEvents(sink)

    found = loxa.expect_event(events, message="hello")
    assert found is not None
    loxa.expect_attr(found, "message", "hello")

    snap = loxa.snapshot_event(found)
    assert "event_id" not in snap
    assert "timestamp" not in snap
    assert isinstance(loxa.mock_sink(), loxa.MemorySink)

    loxa.set_id_generator(lambda: "evt_fixed")
    logger2, sink2 = loxa.TestLogger()
    logger2.info("id-fixed")
    fixed = loxa.DecodeEvents(sink2)[0]
    assert fixed.get("event_id") == "evt_fixed"
    loxa.reset_for_test()
