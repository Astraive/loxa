from __future__ import annotations

import loza


def test_sampling_and_policy_helpers():
    assert loza.SampleByEvent(lambda e: True) is not None
    assert loza.SampleByOutcome("error") is not None
    assert loza.ShouldSample(lambda e: True, None) is True
    assert loza.AllowFields("key1") is not None
    assert loza.BlockFields("secret") is not None

    assert loza.sample_by_event(lambda e: True) is not None
    assert loza.sample_by_outcome("error") is not None
    assert loza.should_sample(lambda e: True, None) is True
    assert loza.allow_fields("key1") is not None
    assert loza.block_fields("secret") is not None
