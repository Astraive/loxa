from __future__ import annotations

import time
from uuid import uuid4

def uuidv7_like(prefix: str = "evt") -> str:
    return f"{prefix}_{int(time.time() * 1000):x}_{uuid4().hex}"
