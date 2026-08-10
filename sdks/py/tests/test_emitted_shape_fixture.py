from __future__ import annotations

import json
from pathlib import Path
from typing import Any

import loza


def _repo_root() -> Path:
    return Path(__file__).resolve().parents[3]


def _fixture() -> dict[str, Any]:
    path = _repo_root() / "spec" / "fixtures" / "emitted-shape" / "structured_http_success.json"
    return json.loads(path.read_text())


def _lookup(value: dict[str, Any], path: str) -> Any:
    current: Any = value
    for segment in path.split("."):
        assert isinstance(current, dict), f"expected dict at {segment} in {path}"
        current = current[segment]
    return current


def test_shared_emitted_shape_fixture() -> None:
    fixture = _fixture()
    logger = loza.New(loza.Test(fixture["params"]["service"]))
    ctx = logger.start_event(
        loza.Params(
            event=fixture["params"]["event"],
            kind=fixture["params"]["kind"],
            service=fixture["params"]["service"],
            method=fixture["params"]["method"],
            path=fixture["params"]["path"],
            route=fixture["params"]["route"],
            status_code=fixture["params"]["status_code"],
        )
    )
    for key, value in fixture["attrs"].items():
        logger.set(ctx, **{key: value})
    logger.finish(ctx, fixture["finish"]["outcome"])

    payload = json.loads(logger.emit(ctx))
    for path in fixture["expected"]["present"]:
        _lookup(payload, path)
    for path, want in fixture["expected"]["equals"].items():
        assert _lookup(payload, path) == want
