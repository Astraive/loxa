#!/usr/bin/env python3
"""generate-parity-report.py -- Read parity manifests from all SDKs and produce a cross-SDK comparison matrix."""

import argparse
import json
import sys
from pathlib import Path


SDKS = ["go", "py", "rs", "js"]
SDK_LABELS = {"go": "Go", "py": "Python", "rs": "Rust", "js": "JavaScript"}


def load_manifest(repo_root: Path, sdk: str) -> dict | None:
    """Load the parity manifest for a given SDK."""
    manifest_path = repo_root / "sdks" / sdk / "docs" / "sdk-parity-manifest.json"
    if not manifest_path.exists():
        return None
    try:
        with open(manifest_path) as f:
            return json.load(f)
    except (json.JSONDecodeError, OSError):
        return None


def collect_features(manifests: dict[str, dict | None]) -> list[str]:
    """Collect all unique feature names across all manifests."""
    features = set()
    for sdk, manifest in manifests.items():
        if manifest is None:
            continue
        if isinstance(manifest, dict):
            # Could be {"features": [...]} or {"feature_name": {...}}
            if "features" in manifest and isinstance(manifest["features"], list):
                for item in manifest["features"]:
                    if isinstance(item, str):
                        features.add(item)
                    elif isinstance(item, dict) and "name" in item:
                        features.add(item["name"])
            else:
                for key in manifest:
                    if key not in ("version", "sdk", "generated"):
                        features.add(key)
        elif isinstance(manifest, list):
            for item in manifest:
                if isinstance(item, str):
                    features.add(item)
                elif isinstance(item, dict) and "name" in item:
                    features.add(item["name"])
    return sorted(features)


def feature_supported(manifest: dict | None, feature: str) -> str:
    """Check if a feature is supported in a manifest. Returns status string."""
    if manifest is None:
        return "N/A"

    if isinstance(manifest, dict):
        if "features" in manifest and isinstance(manifest["features"], list):
            for item in manifest["features"]:
                if isinstance(item, str) and item == feature:
                    return "Y"
                elif isinstance(item, dict) and item.get("name") == feature:
                    status = item.get("status", item.get("supported", True))
                    if isinstance(status, bool):
                        return "Y" if status else "N"
                    return str(status)
        elif feature in manifest:
            val = manifest[feature]
            if isinstance(val, bool):
                return "Y" if val else "N"
            if isinstance(val, dict):
                status = val.get("status", val.get("supported", True))
                if isinstance(status, bool):
                    return "Y" if status else "N"
                return str(status)
            return str(val)

    return "N"


def generate_report(manifests: dict[str, dict | None], features: list[str]) -> str:
    """Generate the markdown parity report."""
    lines = [
        "# SDK Parity Report",
        "",
        f"Generated from parity manifests across {len(SDKs)} SDKs.",
        "",
        "## Feature Matrix",
        "",
    ]

    # Header
    header = "| Feature |"
    separator = "|---------|"
    for sdk in SDKS:
        header += f" {SDK_LABELS[sdk]} |"
        separator += "------|"
    lines.append(header)
    lines.append(separator)

    # Rows
    total = len(features)
    counts = {sdk: 0 for sdk in SDKS}
    for feature in features:
        row = f"| {feature} |"
        for sdk in SDKS:
            status = feature_supported(manifests[sdk], feature)
            if status in ("Y", "yes", "true", "supported", "complete"):
                row += " Y |"
                counts[sdk] += 1
            elif status in ("N/A", "na", "n/a"):
                row += " -- |"
            else:
                row += f" {status} |"
        lines.append(row)

    # Summary
    lines.append("")
    lines.append("## Summary")
    lines.append("")
    lines.append("| SDK | Supported | Total | Coverage |")
    lines.append("|-----|-----------|-------|----------|")
    for sdk in SDKS:
        pct = (counts[sdk] / total * 100) if total > 0 else 0
        lines.append(f"| {SDK_LABELS[sdk]} | {counts[sdk]} | {total} | {pct:.0f}% |")

    lines.append("")
    return "\n".join(lines)


def main():
    parser = argparse.ArgumentParser(description="Generate cross-SDK parity report")
    parser.add_argument("--repo-root", default=None, help="Repository root (default: auto-detect)")
    parser.add_argument("--output", "-o", help="Output file (default: stdout)")
    args = parser.parse_args()

    if args.repo_root:
        repo_root = Path(args.repo_root)
    else:
        # Walk up from script location to find repo root
        repo_root = Path(__file__).resolve().parent.parent

    manifests = {}
    for sdk in SDKS:
        manifests[sdk] = load_manifest(repo_root, sdk)

    # Check if any manifests were found
    found = sum(1 for m in manifests.values() if m is not None)
    if found == 0:
        print("WARNING: No parity manifests found. Checked:", file=sys.stderr)
        for sdk in SDKS:
            print(f"  sdks/{sdk}/docs/sdk-parity-manifest.json", file=sys.stderr)

    features = collect_features(manifests)
    report = generate_report(manifests, features)

    if args.output:
        with open(args.output, "w") as f:
            f.write(report)
        print(f"Report written to {args.output}", file=sys.stderr)
    else:
        print(report)


if __name__ == "__main__":
    main()
