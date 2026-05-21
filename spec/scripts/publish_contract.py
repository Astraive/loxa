#!/usr/bin/env python3
"""Publish generated contract to a local registry directory.

This script copies generated/contract/loxa-contract.json into generated/registry/ with a
manifest that contains sha256 and timestamp. CI can then upload generated/registry/ to S3/CDN.
"""
from __future__ import annotations

import hashlib
import json
from pathlib import Path
import shutil
import time


def sha256_of_path(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        while True:
            chunk = f.read(8192)
            if not chunk:
                break
            h.update(chunk)
    return h.hexdigest()


def main() -> int:
    repo = Path(__file__).resolve().parents[1]
    src = repo / "generated" / "contract" / "loxa-contract.json"
    if not src.exists():
        print("no generated contract found; run codegen first")
        return 2
    dst_dir = repo / "generated" / "registry"
    dst_dir.mkdir(parents=True, exist_ok=True)
    dst = dst_dir / "loxa-contract.json"
    shutil.copy2(src, dst)
    sha = sha256_of_path(dst)
    manifest = {
        "artifact": "loxa-contract.json",
        "sha256": sha,
        "timestamp": int(time.time()),
        "size": dst.stat().st_size,
    }
    manifest_path = dst_dir / "manifest.json"
    manifest_path.write_text(json.dumps(manifest, indent=2), encoding="utf-8")
    print(f"published contract -> {dst} (sha256={sha})")
    print(f"wrote manifest -> {manifest_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
