from __future__ import annotations

import loxa


def test_config_disabled():
    cfg = loxa.Config.disabled()
    assert cfg.environment == "test"
    assert cfg.level == "fatal"


def test_config_from_env(monkeypatch):
    monkeypatch.setenv("LOXA_SERVICE_NAME", "mysvc")
    cfg = loxa.Config.from_env()
    assert cfg.service == "mysvc"


def test_from_env_function():
    cfg = loxa.from_env()
    assert isinstance(cfg, loxa.Config)


def test_config_options_new():
    loxa.WithRelease("v2.0")
    loxa.WithNamespace("prod")
    loxa.WithApiKey("key123")
    loxa.WithOtelBridge(True)
    loxa.WithRetry(True)
    loxa.WithTimeout(5.0)
    loxa.WithQueueSize(4096)
    loxa.WithLogger(None)


def test_snake_config_options():
    loxa.with_release("v2.0")
    loxa.with_namespace("prod")
    loxa.with_api_key("key123")
    loxa.with_otel_bridge(True)
    loxa.with_retry(True)
    loxa.with_timeout(5.0)
    loxa.with_queue_size(4096)
    loxa.with_logger(None)
    loxa.disabled()
    loxa.from_env()
