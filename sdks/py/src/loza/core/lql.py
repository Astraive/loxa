"""Source-forwarding LQL query support for the Loza Python SDK."""

from __future__ import annotations

from typing import Any

import json
import urllib.error
import urllib.request


class QueryResult:
    """Result of a query against the collector."""

    def __init__(self, columns: list[str], rows: list[dict[str, Any]]) -> None:
        self.columns = columns
        self.rows = rows

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
    parameters: dict[str, Any] | None = None,
    limit: int = 1000,
    api_key: str = "",
    timeout: float = 30.0,
) -> QueryResult:
    """Execute LQL source through the collector's server-side compiler."""
    normalized_limit = min(max(int(limit), 1), 1000)
    url = endpoint.rstrip("/") + "/lql/query"
    body = json.dumps({
        "query": lql,
        "parameters": _encode_parameters(parameters),
        "limit": normalized_limit,
    }).encode("utf-8")
    headers = {"Content-Type": "application/json"}
    if api_key:
        headers["X-API-Key"] = api_key
    req = urllib.request.Request(url, data=body, headers=headers, method="POST")
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            data = json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as error:
        raise _decode_query_error(error) from error
    return QueryResult(columns=data.get("columns", []), rows=data.get("rows", []))


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
