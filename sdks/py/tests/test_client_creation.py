from __future__ import annotations

import loza


def test_config_disabled():
    cfg = loza.Config.disabled()
    assert cfg.environment == "test"
    assert cfg.level == "fatal"


def test_config_from_env(monkeypatch):
    monkeypatch.setenv("LOZA_SERVICE_NAME", "mysvc")
    cfg = loza.Config.from_env()
    assert cfg.service == "mysvc"


def test_from_env_function():
    cfg = loza.from_env()
    assert isinstance(cfg, loza.Config)


def test_config_options_new():
    loza.WithRelease("v2.0")
    loza.WithNamespace("prod")
    loza.WithApiKey("key123")
    loza.WithOtelBridge(True)
    loza.WithRetry(True)
    loza.WithTimeout(5.0)
    loza.WithQueueSize(4096)
    loza.WithLogger(None)


def test_snake_config_options():
    loza.with_release("v2.0")
    loza.with_namespace("prod")
    loza.with_api_key("key123")
    loza.with_otel_bridge(True)
    loza.with_retry(True)
    loza.with_timeout(5.0)
    loza.with_queue_size(4096)
    loza.with_logger(None)
    loza.disabled()
    loza.from_env()
