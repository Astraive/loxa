from __future__ import annotations

def extract_headers(headers: dict[str, str]) -> dict[str, str]:
    lower = {k.lower(): v for k, v in headers.items()}
    return {"request_id": lower.get("x-request-id", ""), "trace_id": lower.get("traceparent", "")}

def inject_headers(headers: dict[str, str], request_id: str = "", trace_id: str = "") -> dict[str, str]:
    if request_id: headers["x-request-id"] = request_id
    if trace_id: headers["traceparent"] = trace_id
    return headers
