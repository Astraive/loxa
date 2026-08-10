from __future__ import annotations

from dataclasses import dataclass


@dataclass(slots=True)
class ErrorInfo:
    type: str
    message: str
    code: str = ""
    retryable: bool = False
    stack: str = ""


def extract_error(error: BaseException) -> dict[str, object]:
    return {"type": error.__class__.__name__, "message": str(error)}


class LozaError(Exception):
    """Base class for typed LOZA lifecycle errors."""


class DuplicateEmitError(LozaError):
    """Raised when Emit is called after an event reached emitted."""


class EventClosedError(LozaError):
    """Raised when a closed event is mutated or finished."""


class EventAlreadyFinishedError(LozaError):
    """Raised when Finish or FinishError is called more than once."""


class EventValidationError(LozaError, ValueError):
    """Raised when strict validation rejects an event."""
