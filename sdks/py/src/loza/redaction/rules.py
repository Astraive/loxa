"""Redaction rules - lightweight SDK stub.

Full redaction rule engine lives in the loza-collector package.
"""

from __future__ import annotations

from typing import Any, Callable

Redactor = Callable[[dict[str, Any]], dict[str, Any]]
