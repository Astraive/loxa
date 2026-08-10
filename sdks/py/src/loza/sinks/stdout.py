from __future__ import annotations

import sys
from typing import TextIO


class StdoutSink:
    def __init__(self, stream: TextIO | None = None) -> None:
        self._stream = stream or sys.stdout

    def write(self, encoded: str) -> None:
        print(encoded, file=self._stream)
