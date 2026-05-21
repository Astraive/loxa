#!/usr/bin/env python3
"""Publish contract registry to S3 (prototype).

Requires boto3 available in the runtime and AWS credentials configured via environment
variables or instance profile. This script uploads generated/registry/loxa-contract.json and
manifest.json to the configured S3 bucket.

Environment variables:
- LOXA_CONTRACT_BUCKET (required)
- LOXA_CONTRACT_PREFIX (optional)
- CLOUDFRONT_DISTRIBUTION_ID (optional) - if provided, an invalidation is attempted.

Usage:
  python loxa-spec/scripts/publish_contract_s3.py --dry-run
"""
from __future__ import annotations

import hashlib
import json
import os
import sys
from pathlib import Path
import argparse


def _load_local_registry(spec_root: Path) -> Path:
    registry_dir = spec_root / "generated" / "registry"
    contract = registry_dir / "loxa-contract.json"
    manifest = registry_dir / "manifest.json"
    if not contract.exists() or not manifest.exists():
        print("Local registry artifact missing. Run publish_contract.py first.")
        sys.exit(2)
    return registry_dir


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--dry-run", action="store_true")
    args = parser.parse_args()

    try:
        import boto3
    except Exception:
        print("boto3 is required to publish to S3. Install with: pip install boto3")
        return 3

    spec_root = Path(__file__).resolve().parents[1]
    registry_dir = _load_local_registry(spec_root)

    bucket = os.environ.get("LOXA_CONTRACT_BUCKET")
    if not bucket:
        print("LOXA_CONTRACT_BUCKET must be set to the S3 bucket name")
        return 4
    prefix = os.environ.get("LOXA_CONTRACT_PREFIX", "")
    if prefix and not prefix.endswith("/"):
        prefix = prefix + "/"

    s3 = boto3.client("s3")

    uploaded = []
    for fname in ("loxa-contract.json", "manifest.json"):
        local = registry_dir / fname
        key = f"{prefix}{fname}"
        print(f"Uploading {local} -> s3://{bucket}/{key}")
        if args.dry_run:
            continue
        s3.upload_file(str(local), bucket, key, ExtraArgs={"ContentType": "application/json", "CacheControl": "public, max-age=60"})
        uploaded.append(key)

    distro = os.environ.get("CLOUDFRONT_DISTRIBUTION_ID")
    if distro and not args.dry_run:
        cf = boto3.client("cloudfront")
        print(f"Creating CloudFront invalidation for distribution {distro}")
        paths = {"Quantity": len(uploaded), "Items": [f"/{k}" for k in uploaded]}
        cf.create_invalidation(DistributionId=distro, InvalidationBatch={"Paths": paths, "CallerReference": str(int(time.time()))})
        print("Invalidation requested")

    print("Done")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
