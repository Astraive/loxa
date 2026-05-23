
from loxa.core.config import load_layered_config


def test_load_layered_config_defaults_then_user(tmp_path, monkeypatch):
    defaults = tmp_path / "loxa-py.defaults.yaml"
    user = tmp_path / ".loxa-py.yaml"
    defaults.write_text(
        "\n".join(
            [
                "service: default-service",
                "environment: development",
                "level: info",
                "async_config:",
                "  enabled: false",
                "  queue_size: 8192",
                "security:",
                "  max_event_bytes: 262144",
            ]
        ),
        encoding="utf-8",
    )
    user.write_text("service: user-service\nlevel: debug\n", encoding="utf-8")
    monkeypatch.setenv("LOXA_PY_DEFAULTS", str(defaults))
    monkeypatch.setenv("LOXA_PY_CONFIG", str(user))

    cfg = load_layered_config()

    assert cfg.service == "user-service"
    assert cfg.level == "debug"
    assert cfg.security.max_event_bytes == 262144
