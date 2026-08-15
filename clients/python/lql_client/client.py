from __future__ import annotations

import base64
import json
import os
import socket
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass, field
from enum import StrEnum
from typing import Any


class ErrorCategory(StrEnum):
    INVALID_CONFIGURATION = "invalid_configuration"
    TRANSPORT = "transport"
    AUTHENTICATION = "authentication"
    SCOPE = "scope"
    DIAGNOSTICS = "diagnostics"
    COMPILER_UNAVAILABLE = "compiler_unavailable"
    EXECUTION = "execution"
    TIMEOUT = "timeout"
    MALFORMED_RESPONSE = "malformed_response"


@dataclass(frozen=True)
class QueryValue:
    value: Any
    type: str

    def as_json(self) -> dict[str, Any]:
        return {"type": self.type, "value": self.value}


@dataclass(frozen=True)
class QueryColumn:
    name: str
    type: str = ""
    nullable: bool = False


@dataclass
class QueryResult:
    columns: list[QueryColumn]
    rows: list[dict[str, Any]]
    duration_ms: int = 0
    row_count: int = 0

    def __post_init__(self) -> None:
        if not self.row_count:
            self.row_count = len(self.rows)

    def __len__(self) -> int:
        return len(self.rows)

    def __iter__(self):
        return iter(self.rows)


class QueryError(RuntimeError):
    def __init__(
        self,
        message: str,
        *,
        category: ErrorCategory,
        status: int = 0,
        diagnostics: list[dict[str, Any]] | None = None,
        cause: BaseException | None = None,
    ) -> None:
        super().__init__(message)
        self.category = category
        self.status = status
        self.diagnostics = diagnostics or []
        self.__cause__ = cause


@dataclass
class ConnectionConfig:
    dsn: str = ""
    endpoint: str = ""
    collector: str = ""
    api_key: str = ""
    username: str = ""
    password: str = ""
    env: str = ""
    service: str = ""
    timeout: float = 30.0
    max_response_bytes: int = 8 * 1024 * 1024


@dataclass
class Client:
    _endpoint: str
    _collector: str
    _api_key: str = ""
    _username: str = ""
    _password: str = ""
    _env: str = ""
    _service: str = ""
    _timeout: float = 30.0
    _max_response_bytes: int = 8 * 1024 * 1024

    def __init__(self, config: ConnectionConfig) -> None:
        raw_dsn = config.dsn or os.getenv("LOZA_DSN", "")
        if raw_dsn:
            try:
                parsed = _parse_dsn(raw_dsn)
            except ValueError as error:
                raise QueryError("invalid LQL connection configuration: invalid DSN", category=ErrorCategory.INVALID_CONFIGURATION) from error
            config.endpoint = config.endpoint or parsed["endpoint"]
            config.collector = config.collector or parsed["collector"]
            config.env = config.env or parsed["env"]
            config.service = config.service or parsed["service"]
            if not config.username:
                config.username = parsed["username"]
                if not config.password:
                    config.password = parsed["password"]
        config.api_key = config.api_key or os.getenv("LOZA_API_KEY", "")
        config.username = config.username or os.getenv("LOZA_USERNAME", "")
        config.password = config.password or os.getenv("LOZA_PASSWORD", "")
        endpoint = config.endpoint.rstrip("/")
        parsed_endpoint = urllib.parse.urlsplit(endpoint)
        if parsed_endpoint.scheme not in {"http", "https"} or not parsed_endpoint.netloc or parsed_endpoint.username or parsed_endpoint.password:
            raise QueryError("invalid LQL connection configuration: endpoint must be HTTP(S) without userinfo", category=ErrorCategory.INVALID_CONFIGURATION)
        if not config.collector or not _valid_collector(config.collector):
            raise QueryError("invalid LQL connection configuration: collector slug is required", category=ErrorCategory.INVALID_CONFIGURATION)
        if config.username and not config.password and not config.username.startswith("lx_pub_"):
            raise QueryError("invalid LQL connection configuration: basic username requires a password", category=ErrorCategory.INVALID_CONFIGURATION)
        if config.username and not config.api_key and parsed_endpoint.scheme == "http" and not _is_localhost(parsed_endpoint.hostname or ""):
            raise QueryError("invalid LQL connection configuration: basic authentication requires TLS", category=ErrorCategory.INVALID_CONFIGURATION)
        self._endpoint = endpoint
        self._collector = config.collector
        self._api_key = config.api_key
        self._username = config.username
        self._password = config.password
        self._env = config.env
        self._service = config.service
        self._timeout = config.timeout if config.timeout > 0 else 30.0
        self._max_response_bytes = config.max_response_bytes if config.max_response_bytes > 0 else 8 * 1024 * 1024

    def query(self, source: str, parameters: dict[str, QueryValue | Any] | None = None, limit: int = 1000) -> QueryResult:
        if not source.strip():
            raise QueryError("LQL query source is required", category=ErrorCategory.INVALID_CONFIGURATION)
        limit = max(1, min(int(limit), 1000))
        encoded_parameters = {
            name: value.as_json() if isinstance(value, QueryValue) else {"type": _infer_type(value), "value": value}
            for name, value in (parameters or {}).items()
        }
        body = json.dumps({"query": source, "parameters": encoded_parameters, "limit": limit}, separators=(",", ":")).encode()
        endpoint = f"{self._endpoint}/collectors/{urllib.parse.quote(self._collector, safe='')}/lql/query"
        headers = {"Content-Type": "application/json"}
        if self._api_key:
            headers["Authorization"] = f"Bearer {self._api_key}"
        elif self._username:
            token = base64.b64encode(f"{self._username}:{self._password}".encode()).decode("ascii")
            headers["Authorization"] = f"Basic {token}"
        if self._env:
            headers["X-Loza-Env"] = self._env
        if self._service:
            headers["X-Loza-Service"] = self._service
        request = urllib.request.Request(endpoint, data=body, headers=headers, method="POST")
        try:
            with urllib.request.urlopen(request, timeout=self._timeout) as response:
                raw = response.read(self._max_response_bytes + 1)
                status = response.status
        except urllib.error.HTTPError as error:
            raw = error.read(self._max_response_bytes + 1)
            raise _http_error(error.code, raw) from error
        except (TimeoutError, socket.timeout) as error:
            raise QueryError("LQL query timed out", category=ErrorCategory.TIMEOUT, cause=error) from error
        except urllib.error.URLError as error:
            reason = error.reason
            category = ErrorCategory.TIMEOUT if isinstance(reason, (TimeoutError, socket.timeout)) else ErrorCategory.TRANSPORT
            raise QueryError("LQL query transport failed", category=category, cause=error) from error
        if len(raw) > self._max_response_bytes:
            raise QueryError("LQL response exceeds the configured size limit", category=ErrorCategory.MALFORMED_RESPONSE)
        if not 200 <= status < 300:
            raise _http_error(status, raw)
        try:
            payload = json.loads(raw)
            columns = [_column(item) for item in payload["columns"]]
            rows = payload["rows"]
            if not isinstance(rows, list) or not all(isinstance(row, dict) for row in rows):
                raise ValueError("rows must be an array of objects")
            return QueryResult(columns, rows, int(payload.get("duration_ms", 0)), int(payload.get("row_count", len(rows))))
        except (KeyError, TypeError, ValueError, json.JSONDecodeError) as error:
            raise QueryError("LQL response has an invalid result envelope", category=ErrorCategory.MALFORMED_RESPONSE, cause=error) from error


def _parse_dsn(raw: str) -> dict[str, str]:
    parsed = urllib.parse.urlsplit(raw)
    if parsed.scheme != "loza" or not parsed.hostname:
        raise ValueError("scheme and host required")
    collector = urllib.parse.unquote(parsed.path.lstrip("/"))
    if not collector:
        raise ValueError("collector required")
    username = urllib.parse.unquote(parsed.username or "")
    password = urllib.parse.unquote(parsed.password or "")
    if parsed.username is not None and (not username or (not password and not username.startswith("lx_pub_"))):
        raise ValueError("invalid credentials")
    tls = parsed.query and urllib.parse.parse_qs(parsed.query).get("tls", [""])[0]
    use_tls = False if parsed.hostname in {"localhost", "127.0.0.1", "::1"} else True
    if tls == "true":
        use_tls = True
    elif tls == "false":
        use_tls = False
    elif tls not in {"", "auto"}:
        raise ValueError("invalid tls")
    port = parsed.port or (9308 if parsed.hostname in {"localhost", "127.0.0.1", "::1"} else 443 if use_tls else 80)
    query = urllib.parse.parse_qs(parsed.query)
    return {
        "endpoint": f"{'https' if use_tls else 'http'}://{parsed.hostname}:{port}",
        "collector": collector,
        "env": query.get("env", ["default"])[0] or "default",
        "service": query.get("service", [""])[0],
        "username": username,
        "password": password,
    }


def _valid_collector(value: str) -> bool:
    return len(value) <= 128 and value[0].isalnum() and all(char.isalnum() or char in "_-" for char in value)


def _is_localhost(host: str) -> bool:
    return host in {"localhost", "127.0.0.1", "::1"}


def _infer_type(value: Any) -> str:
    if value is None:
        return "null"
    if isinstance(value, bool):
        return "bool"
    if isinstance(value, int):
        return "int"
    if isinstance(value, float):
        return "float"
    return "string"


def _column(value: Any) -> QueryColumn:
    if isinstance(value, str):
        return QueryColumn(value)
    if isinstance(value, dict) and isinstance(value.get("name"), str):
        return QueryColumn(value["name"], str(value.get("type", "")), bool(value.get("nullable", False)))
    raise ValueError("invalid column")


def _http_error(status: int, raw: bytes) -> QueryError:
    try:
        payload = json.loads(raw)
    except (ValueError, json.JSONDecodeError):
        payload = {}
    message = str(payload.get("error") or payload.get("message") or f"LQL query failed with HTTP {status}")
    category = {
        400: ErrorCategory.DIAGNOSTICS,
        401: ErrorCategory.AUTHENTICATION,
        403: ErrorCategory.SCOPE,
        503: ErrorCategory.COMPILER_UNAVAILABLE,
    }.get(status, ErrorCategory.EXECUTION)
    return QueryError(message, category=category, status=status, diagnostics=payload.get("diagnostics", []))
