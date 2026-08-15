"""Tests for create_loza and alias functions."""

import json

import loza


def test_alias_creates_logger_with_different_service():
    logger = loza.alias("audit")
    assert isinstance(logger, loza.Logger)


def test_create_loza_returns_logger():
    logger = loza.create_loza(service="test-svc")
    assert isinstance(logger, loza.Logger)


def test_logger_alias_does_not_mutate_original():
    original = loza.default()
    aliased = original.alias("other")
    assert aliased is not original


def test_alias_preserves_config():
    logger = loza.create_loza(service="api")
    aliased = logger.alias("audit")
    assert isinstance(aliased, loza.Logger)


def test_alias_emits_metadata_without_changing_service():
    sink = loza.MemorySink()
    logger = loza.Logger(loza.Config(service="api", environment="test", sinks=[sink]))
    aliased = logger.alias("audit")
    ctx = aliased.start_event(loza.Params(event="permission.changed"))
    aliased.finish(ctx, "success")
    payload = json.loads(aliased.emit(ctx))
    assert payload["service"] == "api"
    assert payload["attrs"]["loza"]["alias"] == "audit"


def test_uppercase_aliases():
    assert loza.CreateLoza is loza.create_loza
    assert loza.Alias is loza.alias
