"""Source-forwarding LQL query support for the Loza Python SDK."""

from typing import Any

import base64
import json
import urllib.error
import urllib.parse
import urllib.request

class QueryResult:
    """Result of a query against the collector."""

    def __init__(self, columns: list[str], rows: list[dict[str, Any]], *, duration_ms: int = 0, row_count: int | None = None) -> None:
        self.columns = columns
        self.rows = rows
        self.duration_ms = duration_ms
        self.row_count = len(rows) if row_count is None else row_count

    def __len__(self) -> int:
        return len(self.rows)

    def __iter__(self):
        return iter(self.rows)

    def __repr__(self) -> str:
        return f"QueryResult(columns={len(self.columns)}, rows={len(self.rows)})"

class QueryValue:
    """Typed value sent to the LQL compiler."""

    def __init__(self, value: Any, type: str | None = None) -> None:
        self.value = value
        self.type = type

    def as_json(self) -> dict[str, Any]:
        return {"type": self.type, "value": self.value} if self.type else {"value": self.value}


class LQLCompilationError(RuntimeError):
    """Structured diagnostics returned when LQL compilation fails."""

    def __init__(self, message: str, *, status: int = 0, diagnostics: list[dict[str, Any]] | None = None) -> None:
        super().__init__(message)
        self.status = status
        self.diagnostics = diagnostics or []


def _encode_parameters(parameters: dict[str, Any] | None) -> dict[str, Any]:
    encoded: dict[str, Any] = {}
    for name, value in (parameters or {}).items():
        if isinstance(value, QueryValue):
            encoded[name] = value.as_json()
        elif isinstance(value, dict) and "value" in value:
            encoded[name] = value
        else:
            encoded[name] = {"value": value}
    return encoded


def _decode_query_error(error: urllib.error.HTTPError) -> LQLCompilationError:
    try:
        data = json.loads(error.read().decode("utf-8"))
    except (OSError, ValueError):
        return LQLCompilationError(f"lql query failed with HTTP {error.code}", status=error.code)
    message = str(data.get("error") or f"lql query failed with HTTP {error.code}")
    return LQLCompilationError(message, status=error.code, diagnostics=data.get("diagnostics", []))
def query_lql(
    endpoint: str,
    lql: str,
    *,
    collector: str = "",
    parameters: dict[str, Any] | None = None,
    limit: int = 1000,
    api_key: str = "",
    username: str = "",
    password: str = "",
    env: str = "",
    service: str = "",
    timeout: float = 30.0,
) -> QueryResult:
    """Execute LQL source through the collector's server-side compiler."""
    normalized_limit = min(max(int(limit), 1), 1000)
    route = f"/collectors/{urllib.parse.quote(collector, safe='')}/lql/query" if collector else "/lql/query"
    url = endpoint.rstrip("/") + route
    body = json.dumps({"query": lql, "parameters": _encode_parameters(parameters), "limit": normalized_limit}).encode("utf-8")
    headers = {"Content-Type": "application/json"}
    if api_key:
        headers["Authorization"] = f"Bearer {api_key}"
    elif username:
        token = base64.b64encode(f"{username}:{password}".encode("utf-8")).decode("ascii")
        headers["Authorization"] = f"Basic {token}"
    if env:
        headers["X-Loza-Env"] = env
    if service:
        headers["X-Loza-Service"] = service
    req = urllib.request.Request(url, data=body, headers=headers, method="POST")
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            raw = resp.read(8 * 1024 * 1024 + 1)
            if len(raw) > 8 * 1024 * 1024:
                raise LQLCompilationError("lql response exceeds the configured size limit", status=resp.status)
            data = json.loads(raw.decode("utf-8"))
    except urllib.error.HTTPError as error:
        raise _decode_query_error(error) from error
    except (OSError, ValueError) as error:
        raise LQLCompilationError("lql query transport or response decoding failed") from error
    columns = [column if isinstance(column, str) else column.get("name", "") for column in data.get("columns", [])]
    rows = data.get("rows", [])
    if not isinstance(columns, list) or not isinstance(rows, list):
        raise LQLCompilationError("lql response has an invalid result envelope")
    return QueryResult(columns=columns, rows=rows, duration_ms=int(data.get("duration_ms", 0)), row_count=int(data.get("row_count", len(rows))))

def query_sql(
    endpoint: str, sql: str, *, engine: str = "duckdb", api_key: str = "", timeout: float = 30.0
) -> QueryResult:
    """Execute explicit raw SQL through the collector's /query endpoint."""
    url = endpoint.rstrip("/") + "/query"
    body = json.dumps({"query": sql, "engine": engine}).encode("utf-8")
    headers = {"Content-Type": "application/json"}
    if api_key:
        headers["X-API-Key"] = api_key
    req = urllib.request.Request(url, data=body, headers=headers, method="POST")
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        data = json.loads(resp.read().decode("utf-8"))
    return QueryResult(columns=data.get("columns", []), rows=data.get("rows", []))
