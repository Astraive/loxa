import os
import sys
from ..sinks import HTTPBatchSink, StdoutSink


def StderrSink():
    return StdoutSink(sys.stderr)


class RotatingFileSink:
    """Size-based rotating file sink."""

    def __init__(self, path: str = "loxa.log", max_bytes: int = 10 * 1024 * 1024, max_backups: int = 5) -> None:
        self.path = path
        self.max_bytes = max_bytes
        self.max_backups = max_backups
        os.makedirs(os.path.dirname(os.path.abspath(path)), exist_ok=True)
        self._file = open(path, "a", encoding="utf-8")
        self._size = os.path.getsize(path) if os.path.exists(path) else 0

    def write(self, encoded: str) -> None:
        line = encoded + "\n"
        size = len(line.encode("utf-8"))
        if self._size + size > self.max_bytes:
            self._rotate()
        self._file.write(line)
        self._file.flush()
        self._size += size

    def _rotate(self) -> None:
        self._file.close()
        for i in range(self.max_backups - 1, 0, -1):
            src = f"{self.path}.{i}"
            dst = f"{self.path}.{i + 1}"
            if os.path.exists(src):
                if i + 1 >= self.max_backups:
                    os.remove(src)
                else:
                    os.rename(src, dst)
        if os.path.exists(self.path):
            os.rename(self.path, f"{self.path}.1")
        self._file = open(self.path, "a", encoding="utf-8")
        self._size = 0

    def flush(self) -> None:
        self._file.flush()

    def close(self) -> None:
        self._file.close()


def CollectorSink(config=None):
    endpoint = (
        getattr(config, "endpoint", "http://127.0.0.1:9308/events")
        if config is not None
        else "http://127.0.0.1:9308/events"
    )
    return HTTPBatchSink(endpoint)
