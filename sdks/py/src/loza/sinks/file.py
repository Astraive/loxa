from __future__ import annotations

from pathlib import Path
from typing import TextIO


def _safe_output_path(path: str) -> Path:
    if "\x00" in path:
        raise ValueError("file sink path contains a null byte")
    candidate = Path(path)
    if any(part == ".." for part in candidate.parts):
        raise ValueError("file sink path must not contain parent-directory traversal")
    if candidate.exists() and candidate.is_dir():
        raise ValueError("file sink path must be a file")
    return candidate


class FileSink:
    def __init__(self, path: str) -> None:
        self.path = _safe_output_path(path)
        self.path.parent.mkdir(parents=True, exist_ok=True)
        self._file: TextIO = self.path.open("a", encoding="utf-8")

    def write(self, encoded: str) -> None:
        self._file.write(encoded)
        self._file.write("\n")

    def flush(self) -> None:
        self._file.flush()

    def close(self) -> None:
        self._file.close()
