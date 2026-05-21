from datetime import datetime

def NDJSONArchiveKey(prefix: str, ts: datetime) -> str:
    return f"{prefix.rstrip('/')}/year={ts:%Y}/month={ts:%m}/day={ts:%d}/hour={ts:%H}/events.ndjson"
