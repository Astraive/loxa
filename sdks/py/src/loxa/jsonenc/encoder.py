import json
from typing import Any

def encode(value: Any, pretty: bool = False) -> str:
    return json.dumps(value, indent=2 if pretty else None, separators=None if pretty else (",", ":"), sort_keys=True)
