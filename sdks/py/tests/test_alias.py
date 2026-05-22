"""Tests for create_loxa and alias functions."""
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


def test_uppercase_aliases():
    assert loxa.CreateLoxa is loxa.create_loxa
    assert loxa.Alias is loxa.alias
