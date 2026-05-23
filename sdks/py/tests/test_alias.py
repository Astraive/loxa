"""Tests for create_loxa and alias functions."""
import json

import loxa


def test_alias_creates_logger_with_different_service():
    logger = loxa.alias("audit")
    assert isinstance(logger, loxa.Logger)


def test_create_loxa_returns_logger():
    logger = loxa.create_loxa(service="test-svc")
    assert isinstance(logger, loxa.Logger)


def test_logger_alias_does_not_mutate_original():
    original = loxa.default()
    aliased = original.alias("other")
    assert aliased is not original


def test_alias_preserves_config():
    logger = loxa.create_loxa(service="api")
    aliased = logger.alias("audit")
    assert isinstance(aliased, loxa.Logger)


def test_alias_emits_metadata_without_changing_service():
    sink = loxa.MemorySink()
    logger = loxa.Logger(loxa.Config(service="api", environment="test", sinks=[sink]))
    aliased = logger.alias("audit")
    ctx = aliased.start_event(loxa.Params(event="permission.changed"))
    aliased.finish(ctx, "success")
    payload = json.loads(aliased.emit(ctx))
    assert payload["service"] == "api"
    assert payload["attrs"]["loxa"]["alias"] == "audit"


def test_uppercase_aliases():
    assert loxa.CreateLoxa is loxa.create_loxa
    assert loxa.Alias is loxa.alias
