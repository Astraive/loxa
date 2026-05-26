#!/usr/bin/env python3
from __future__ import annotations

import argparse
from pathlib import Path

from emitters.conformance import render_conformance_manifest
from emitters.contract_json import render_contract_json
from emitters.cortex_contract import render_cortex_contract_json
from emitters.go import render_go_contract
from emitters.lql_schema import render_lql_schema
from emitters.python import render_python_contract
from emitters.rust import render_rust_contract
from model import build_contract, build_cortex_contract


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--spec-root", default=str(Path(__file__).resolve().parents[1]))
    parser.add_argument("--check", action="store_true", help="Verify generated artifacts are up-to-date without writing")
    args = parser.parse_args()

    spec_root = Path(args.spec_root).resolve()
    loxa_contract = build_contract(spec_root)
    cortex_contract = build_cortex_contract(spec_root)

    outputs = {
        spec_root / "conformance" / "manifest.json": render_conformance_manifest(loxa_contract),
        spec_root / "generated" / "contract" / "loxa-contract.json": render_contract_json(loxa_contract),
        spec_root / "generated" / "contract" / "cortex-contract.json": render_cortex_contract_json(cortex_contract),
        spec_root / "generated" / "contract" / "conformance_manifest.json": render_conformance_manifest(loxa_contract),
        spec_root / "generated" / "conformance_manifest.json": render_conformance_manifest(loxa_contract),
        spec_root / "generated" / "go" / "contract" / "contract.go": render_go_contract(loxa_contract),
        spec_root / "generated" / "python" / "loxa_contract.py": render_python_contract(loxa_contract),
        spec_root / "generated" / "rust" / "contract.rs": render_rust_contract(loxa_contract),
        spec_root / "generated" / "lql" / "schema.rs": render_lql_schema(loxa_contract),
    }

    failed = False
    stale_count = 0
    
    for path, content in outputs.items():
        if args.check:
            if not path.exists():
                print(f"missing generated artifact: {path}")
                failed = True
                stale_count += 1
            elif path.read_text(encoding="utf-8") != content:
                print(f"stale generated artifact: {path}")
                failed = True
                stale_count += 1
        else:
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(content, encoding="utf-8")

    if args.check:
        if failed:
            print(f"\n{stale_count} generated artifact(s) are out of date. Run: python codegen/generate.py")
        else:
            print("✓ all generated artifacts are up-to-date")

    return 1 if failed else 0


if __name__ == "__main__":
    raise SystemExit(main())
