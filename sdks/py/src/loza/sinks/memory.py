class MemorySink:
    def __init__(self) -> None:
        self.events: list[str] = []

    def write(self, encoded: str) -> None:
        self.events.append(encoded)

    def flush(self) -> None:
        return None

    def close(self) -> None:
        return None
