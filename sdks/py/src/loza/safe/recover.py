from __future__ import annotations

import traceback
from contextlib import contextmanager
from dataclasses import dataclass
from typing import Callable, Iterator


@dataclass(slots=True)
class RecoveredPanic:
    error: BaseException
    stack: str


@contextmanager
def recover(handler: Callable[[RecoveredPanic], None] | None = None, *, reraise: bool = True) -> Iterator[None]:
    try:
        yield
    except Exception as exc:
        recovered = RecoveredPanic(exc, traceback.format_exc())
        if handler is not None:
            handler(recovered)
        if reraise:
            raise


def run_safely(fn: Callable[[], object], handler: Callable[[RecoveredPanic], None] | None = None):
    with recover(handler, reraise=False):
        return fn()
    return None
