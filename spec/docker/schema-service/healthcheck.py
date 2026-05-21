#!/usr/bin/env python3
from __future__ import annotations

import os
import sys
import urllib.error
import urllib.request


def main() -> int:
    host = os.getenv("SCHEMA_SERVICE_HOST", "127.0.0.1")
    if host == "0.0.0.0":
        host = "127.0.0.1"
    port = os.getenv("SCHEMA_SERVICE_PORT", "8080")
    try:
        with urllib.request.urlopen(f"http://{host}:{port}/health", timeout=2) as response:
            return 0 if response.status == 200 else 1
    except (urllib.error.URLError, TimeoutError):
        return 1


if __name__ == "__main__":
    sys.exit(main())
