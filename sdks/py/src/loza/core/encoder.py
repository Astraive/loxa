from __future__ import annotations

import json
from typing import Any


def JSONEncoder(payload: dict[str, Any]) -> str:
    return json.dumps(payload, separators=(",", ":"), sort_keys=True)


def PrettyJSONEncoder(payload: dict[str, Any]) -> str:
    return json.dumps(payload, indent=2, sort_keys=True)
