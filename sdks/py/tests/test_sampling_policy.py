from __future__ import annotations

import loxa


def test_sampling_and_policy_helpers():
    assert loxa.SampleByEvent(lambda e: True) is not None
    assert loxa.SampleByOutcome("error") is not None
    assert loxa.ShouldSample(lambda e: True, None) is True
    assert loxa.AllowFields("key1") is not None
    assert loxa.BlockFields("secret") is not None

    assert loxa.sample_by_event(lambda e: True) is not None
    assert loxa.sample_by_outcome("error") is not None
    assert loxa.should_sample(lambda e: True, None) is True
    assert loxa.allow_fields("key1") is not None
    assert loxa.block_fields("secret") is not None
