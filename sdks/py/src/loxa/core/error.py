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


class LoxaError(Exception):
    """Base class for typed LOXA lifecycle errors."""


class DuplicateEmitError(LoxaError):
    """Raised when Emit is called after an event reached emitted."""


class EventClosedError(LoxaError):
    """Raised when a closed event is mutated or finished."""


class EventAlreadyFinishedError(LoxaError):
    """Raised when Finish or FinishError is called more than once."""


class EventValidationError(LoxaError, ValueError):
    """Raised when strict validation rejects an event."""
