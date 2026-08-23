from __future__ import annotations

import base64
import io
import json
import socket
import urllib.error
from typing import Any

import pytest

from lql_client import Client, ConnectionConfig, ErrorCategory, QueryColumn, QueryError, QueryResult, QueryValue
from lql_client.client import _parse_dsn, _valid_collector


class FakeResponse:
    def __init__(self, payload: bytes, status: int = 200) -> None:
        self.payload = payload
        self.status = status

    def __enter__(self) -> "FakeResponse":
        return self

    def __exit__(self, *_args: object) -> None:
        return None

    def read(self, _limit: int = -1) -> bytes:
        return self.payload


def make_client(**kwargs: Any) -> Client:
    config = ConnectionConfig(endpoint="https://collector.example", collector="demo", **kwargs)
    return Client(config)


def install_response(monkeypatch: pytest.MonkeyPatch, payload: bytes, status: int = 200) -> list[Any]:
    captured: list[Any] = []

    def urlopen(request: Any, timeout: float) -> FakeResponse:
        captured.append((request, timeout))
        return FakeResponse(payload, status)

    monkeypatch.setattr("lql_client.client.urllib.request.urlopen", urlopen)
    return captured


def test_query_models_expose_serialization_and_collection_protocols() -> None:
    assert QueryValue("seven", "string").as_json() == {"type": "string", "value": "seven"}

    result = QueryResult([QueryColumn("id")], [{"id": "evt-1"}])
    assert result.row_count == 1
    assert len(result) == 1
    assert list(result) == [{"id": "evt-1"}]

    explicit = QueryResult([], [], duration_ms=4, row_count=0)
    assert explicit.duration_ms == 4
    assert explicit.row_count == 0


@pytest.mark.parametrize(
    ("dsn", "endpoint"),
    [
        ("loza://team.example/events?env=prod&service=worker", "https://team.example:443"),
        ("loza://localhost/events", "http://localhost:9308"),
        ("loza://127.0.0.1/events?tls=true", "https://127.0.0.1:9308"),
        ("loza://127.0.0.1/events?tls=false", "http://127.0.0.1:9308"),
        ("loza://team.example/events?tls=false", "http://team.example:80"),
        ("loza://team.example/events?tls=auto", "https://team.example:443"),
    ],
)
def test_dsn_normalization_selects_tls_and_default_ports(dsn: str, endpoint: str) -> None:
    parsed = _parse_dsn(dsn)
    assert parsed["endpoint"] == endpoint
    assert parsed["collector"] == "events"


def test_dsn_normalization_decodes_credentials_and_query_options() -> None:
    parsed = _parse_dsn("loza://user%40name:p%40ss@team.example/team-prod?env=prod&service=worker")
    assert parsed == {
        "endpoint": "https://team.example:443",
        "collector": "team-prod",
        "env": "prod",
        "service": "worker",
        "username": "user@name",
        "password": "p@ss",
    }


def test_dsn_accepts_public_username_without_password() -> None:
    parsed = _parse_dsn("loza://lx_pub_abc@team.example/events")
    assert parsed["username"] == "lx_pub_abc"
    assert parsed["password"] == ""


@pytest.mark.parametrize(
    "dsn",
    [
        "https://team.example/events",
        "loza:///events",
        "loza://team.example/",
        "loza://user@team.example/events",
        "loza://user:@team.example/events",
        "loza://team.example/events?tls=maybe",
    ],
)
def test_invalid_dsn_values_are_rejected(dsn: str) -> None:
    with pytest.raises(ValueError):
        _parse_dsn(dsn)


def test_client_applies_dsn_and_environment_defaults(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("LOZA_DSN", "loza://dsn.example/from-dsn?env=dsn-env&service=dsn-service")
    monkeypatch.setenv("LOZA_API_KEY", "env-api")
    monkeypatch.setenv("LOZA_USERNAME", "env-user")
    monkeypatch.setenv("LOZA_PASSWORD", "env-pass")
    config = ConnectionConfig(timeout=0, max_response_bytes=0)

    client = Client(config)

    assert client._endpoint == "https://dsn.example:443"
    assert client._collector == "from-dsn"
    assert client._api_key == "env-api"
    assert client._username == "env-user"
    assert client._password == "env-pass"
    assert client._env == "dsn-env"
    assert client._service == "dsn-service"
    assert client._timeout == 30.0
    assert client._max_response_bytes == 8 * 1024 * 1024


def test_client_configuration_overrides_dsn_values() -> None:
    client = Client(
        ConnectionConfig(
            dsn="loza://dsn-user:dsn-pass@dsn.example/dsn-collector?env=dsn-env&service=dsn-service",
            endpoint="https://override.example/",
            collector="override",
            env="override-env",
            service="override-service",
            username="config-user",
            password="config-pass",
        )
    )

    assert client._endpoint == "https://override.example"
    assert client._collector == "override"
    assert client._env == "override-env"
    assert client._service == "override-service"
    assert client._username == "config-user"
    assert client._password == "config-pass"


def test_invalid_dsn_is_wrapped_as_configuration_error() -> None:
    with pytest.raises(QueryError) as caught:
        Client(ConnectionConfig(dsn="not-a-loza-dsn"))
    assert caught.value.category == ErrorCategory.INVALID_CONFIGURATION
    assert isinstance(caught.value.__cause__, ValueError)


@pytest.mark.parametrize(
    "config",
    [
        ConnectionConfig(endpoint="ftp://collector.example", collector="demo"),


        ConnectionConfig(endpoint="https://user:pass@collector.example", collector="demo"),
        ConnectionConfig(endpoint="https://collector.example", collector="$demo"),
        ConnectionConfig(endpoint="https://collector.example", collector="bad slug"),
        ConnectionConfig(endpoint="https://collector.example", collector="x" * 129),
        ConnectionConfig(endpoint="https://collector.example", collector="demo", username="user"),
        ConnectionConfig(endpoint="http://collector.example", collector="demo", username="user", password="pass"),
    ],
)
def test_invalid_client_configuration_is_categorized(config: ConnectionConfig) -> None:
    with pytest.raises(QueryError) as caught:
        Client(config)
    assert caught.value.category == ErrorCategory.INVALID_CONFIGURATION

def test_client_uses_dsn_username_when_password_is_already_configured() -> None:
    client = Client(
        ConnectionConfig(
            dsn="loza://dsn-user:dsn-pass@dsn.example/collector",
            password="config-pass",
        )
    )
    assert client._username == "dsn-user"
    assert client._password == "config-pass"


def test_query_rejects_blank_source() -> None:
    with pytest.raises(QueryError) as caught:
        make_client().query("  ")
    assert caught.value.category == ErrorCategory.INVALID_CONFIGURATION


def test_localhost_basic_auth_is_allowed_and_uses_basic_header(monkeypatch: pytest.MonkeyPatch) -> None:
    captured = install_response(monkeypatch, b'{"columns":[],"rows":[]}')
    client = Client(ConnectionConfig(endpoint="http://localhost", collector="demo", username="user", password="pass"))

    result = client.query("from events")

    assert result.rows == []
    request, timeout = captured[0]
    assert timeout == 30.0
    assert request.headers["Authorization"] == "Basic " + base64.b64encode(b"user:pass").decode("ascii")


def test_query_serializes_parameters_clamps_limit_and_encodes_headers(monkeypatch: pytest.MonkeyPatch) -> None:
    payload = json.dumps(
        {
            "columns": ["id", {"name": "count", "type": "int", "nullable": True}],
            "rows": [{"id": "evt-1", "count": 1}],
            "duration_ms": 7,
        }
    ).encode()
    captured = install_response(monkeypatch, payload)
    client = Client(
        ConnectionConfig(
            endpoint="https://collector.example/",
            collector="demo",
            api_key="api",
            env="prod",
            service="cli",
            timeout=2.5,
        )
    )

    result = client.query(
        "from events | where id = $id",
        {"id": QueryValue("evt-1", "string"), "none": None, "flag": True, "count": 2, "ratio": 1.5, "text": "x"},
        5000,
    )

    assert result.columns == [QueryColumn("id"), QueryColumn("count", "int", True)]
    assert result.row_count == 1
    request, timeout = captured[0]
    assert timeout == 2.5
    assert request.headers["Authorization"] == "Bearer api"
    assert request.headers["X-loza-env"] == "prod"
    assert request.headers["X-loza-service"] == "cli"
    body = json.loads(request.data)
    assert body["limit"] == 1000
    assert body["parameters"] == {
        "id": {"type": "string", "value": "evt-1"},
        "none": {"type": "null", "value": None},
        "flag": {"type": "bool", "value": True},
        "count": {"type": "int", "value": 2},
        "ratio": {"type": "float", "value": 1.5},
        "text": {"type": "string", "value": "x"},
    }


def test_query_sends_low_limit_as_one(monkeypatch: pytest.MonkeyPatch) -> None:
    captured = install_response(monkeypatch, b'{"columns":[],"rows":[]}')
    make_client().query("from events", limit=0)
    assert json.loads(captured[0][0].data)["limit"] == 1


@pytest.mark.parametrize(
    ("exception", "category"),
    [
        (TimeoutError("slow"), ErrorCategory.TIMEOUT),
        (socket.timeout("slow"), ErrorCategory.TIMEOUT),
        (urllib.error.URLError(TimeoutError("slow")), ErrorCategory.TIMEOUT),
        (urllib.error.URLError("offline"), ErrorCategory.TRANSPORT),
    ],
)
def test_query_maps_transport_failures(monkeypatch: pytest.MonkeyPatch, exception: BaseException, category: ErrorCategory) -> None:
    def urlopen(_request: Any, timeout: float) -> FakeResponse:
        raise exception

    monkeypatch.setattr("lql_client.client.urllib.request.urlopen", urlopen)
    with pytest.raises(QueryError) as caught:
        make_client().query("from events")
    assert caught.value.category == category
    assert caught.value.__cause__ is exception


def test_query_rejects_oversized_response(monkeypatch: pytest.MonkeyPatch) -> None:
    install_response(monkeypatch, b"0123456789")
    with pytest.raises(QueryError) as caught:
        Client(ConnectionConfig(endpoint="https://collector.example", collector="demo", max_response_bytes=5)).query("from events")
    assert caught.value.category == ErrorCategory.MALFORMED_RESPONSE
    assert "size limit" in str(caught.value)


def test_query_maps_non_http_error_status_from_response(monkeypatch: pytest.MonkeyPatch) -> None:
    install_response(monkeypatch, b'{"message":"execution failed","diagnostics":[{"line":1}]}', status=500)
    with pytest.raises(QueryError) as caught:
        make_client().query("from events")
    assert caught.value.category == ErrorCategory.EXECUTION
    assert caught.value.status == 500
    assert caught.value.diagnostics == [{"line": 1}]


@pytest.mark.parametrize(
    ("status", "payload", "category", "message"),
    [
        (400, b'{"error":"bad query","diagnostics":[]}', ErrorCategory.DIAGNOSTICS, "bad query"),
        (401, b'{"message":"unauthorized"}', ErrorCategory.AUTHENTICATION, "unauthorized"),
        (403, b"not-json", ErrorCategory.SCOPE, "LQL query failed with HTTP 403"),
        (503, b'{"error":"compiler down"}', ErrorCategory.COMPILER_UNAVAILABLE, "compiler down"),
        (500, b'{"message":"boom"}', ErrorCategory.EXECUTION, "boom"),
    ],
)
def test_query_maps_http_error_statuses(status: int, payload: bytes, category: ErrorCategory, message: str, monkeypatch: pytest.MonkeyPatch) -> None:
    def urlopen(_request: Any, timeout: float) -> Any:
        raise urllib.error.HTTPError("https://collector.example", status, "failed", {}, io.BytesIO(payload))

    monkeypatch.setattr("lql_client.client.urllib.request.urlopen", urlopen)
    with pytest.raises(QueryError) as caught:
        make_client().query("from events")
    assert caught.value.category == category
    assert caught.value.status == status
    assert str(caught.value) == message


@pytest.mark.parametrize(
    "payload",
    [
        b"",
        b"{}",
        b'{"columns":[]}',
        b'{"columns":[],"rows":{}}',
        b'{"columns":[],"rows":[1]}',
        b'{"columns":[1],"rows":[]}',
        b'{"columns":[{"type":"string"}],"rows":[]}',
        b'{"columns":[],"rows":[],"duration_ms":"slow"}',
        b'{"columns":[],"rows":[],"row_count":"many"}',
    ],
)
def test_query_rejects_empty_and_malformed_result_envelopes(monkeypatch: pytest.MonkeyPatch, payload: bytes) -> None:
    install_response(monkeypatch, payload)
    with pytest.raises(QueryError) as caught:
        make_client().query("from events")
    assert caught.value.category == ErrorCategory.MALFORMED_RESPONSE
    assert isinstance(caught.value.__cause__, (KeyError, TypeError, ValueError))


def test_query_decodes_structured_columns_and_empty_rows(monkeypatch: pytest.MonkeyPatch) -> None:
    payload = b'{"columns":[{"name":"name"}],"rows":[],"row_count":0}'
    install_response(monkeypatch, payload)
    result = make_client().query("from events", parameters={}, limit=10)
    assert result.columns == [QueryColumn("name", "", False)]
    assert result.rows == []
    assert result.row_count == 0


def test_collector_validation_and_query_error_fields() -> None:
    assert _valid_collector("demo")
    assert not _valid_collector("-demo")
    assert not _valid_collector("demo space")

    cause = ValueError("bad")
    error = QueryError("failed", category=ErrorCategory.EXECUTION, status=500, diagnostics=[{"x": 1}], cause=cause)
    assert error.category == ErrorCategory.EXECUTION
    assert error.status == 500
    assert error.diagnostics == [{"x": 1}]
    assert error.__cause__ is cause
