"""LQL query support for the Loza Python SDK.

Sends LQL queries to the collector which compiles them to SQL server-side.
"""

from __future__ import annotations

from typing import Any

import json
import urllib.request


class QueryResult:
    """Result of a LQL or SQL query against the collector."""

    def __init__(self, columns: list[str], rows: list[dict[str, Any]], sql: str = "") -> None:
        self.columns = columns
        self.rows = rows
        self.sql = sql

    def __len__(self) -> int:
        return len(self.rows)

    def __iter__(self):
        return iter(self.rows)

    def __repr__(self) -> str:
        return f"QueryResult(columns={len(self.columns)}, rows={len(self.rows)})"


def query_lql(endpoint: str, lql: str, *, api_key: str = "", timeout: float = 30.0) -> QueryResult:
    """Execute a LQL query against the collector.

    The collector compiles LQL to SQL server-side and returns results.

    Args:
        endpoint: Collector URL (e.g. "http://localhost:9308")
        lql: LQL query string (e.g. 'from events | where level = "error" | limit 10')
        api_key: Optional API key for authentication
        timeout: Request timeout in seconds

    Returns:
        QueryResult with columns, rows, and compiled SQL
    """
    url = endpoint.rstrip("/") + "/lql/query"
    body = json.dumps({"query": lql}).encode("utf-8")

    headers = {"Content-Type": "application/json"}
    if api_key:
        headers["X-API-Key"] = api_key

    req = urllib.request.Request(url, data=body, headers=headers, method="POST")
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        data = json.loads(resp.read().decode("utf-8"))

    return QueryResult(
        columns=data.get("columns", []),
        rows=data.get("rows", []),
        sql=data.get("sql", ""),
    )


def query_sql(
    endpoint: str, sql: str, *, engine: str = "duckdb", api_key: str = "", timeout: float = 30.0
) -> QueryResult:
    """Execute a raw SQL query against the collector.

    Args:
        endpoint: Collector URL
        sql: Raw SQL query
        engine: Query engine ("duckdb" or "clickhouse")
        api_key: Optional API key
        timeout: Request timeout in seconds

    Returns:
        QueryResult with columns, rows, and compiled SQL
    """
    url = endpoint.rstrip("/") + "/query"
    body = json.dumps({"query": sql, "engine": engine}).encode("utf-8")

    headers = {"Content-Type": "application/json"}
    if api_key:
        headers["X-API-Key"] = api_key

    req = urllib.request.Request(url, data=body, headers=headers, method="POST")
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        data = json.loads(resp.read().decode("utf-8"))

    return QueryResult(
        columns=data.get("columns", []),
        rows=data.get("rows", []),
        sql=data.get("sql", ""),
    )
