from __future__ import annotations

from pathlib import Path
from typing import TextIO


class FileSink:
    def __init__(self, path: str) -> None:
        self.path = Path(path)
        self.path.parent.mkdir(parents=True, exist_ok=True)
        self._file: TextIO = self.path.open("a", encoding="utf-8")

    def write(self, encoded: str) -> None:
        self._file.write(encoded)
        self._file.write("\n")

    def flush(self) -> None:
        self._file.flush()

    def close(self) -> None:
        self._file.close()
