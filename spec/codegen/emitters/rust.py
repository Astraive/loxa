from __future__ import annotations

import json
from typing import Any


def render_rust_contract(contract: dict[str, Any]) -> str:
    return (
        "use serde::{Deserialize, Serialize};\n"
        "pub const CONTRACT_JSON: &str = r#\"" + json.dumps(contract, indent=2).replace("\\", "\\\\").replace('"', '\\"') + "\"#;\n"
        "pub const LOZA_SPEC_VERSION: &str = \"{}\";\n".format(contract["spec_version"])
        + "pub const LOZA_INGEST_API_VERSION: &str = \"{}\";\n".format(contract["api_version"])
        + "pub const LOZA_EVENT_VERSION: &str = \"{}\";\n".format(contract["event_version"])
        + "pub const MAX_EVENT_BYTES: usize = {};\n".format(contract["limits"]["max_event_size_bytes"])
        + "#[derive(Clone, Debug, Default, Eq, PartialEq, Serialize, Deserialize)]\n"
        + "pub struct ValidationError { pub field: String, pub code: String, pub message: String, #[serde(default)] pub event_id: String, #[serde(default)] pub retryable: bool }\n"
        + "#[derive(Clone, Debug, Default, Eq, PartialEq, Serialize, Deserialize)]\n"
        + "pub struct CollectorAck { #[serde(default)] pub event_id: String, #[serde(default)] pub status: String, #[serde(default)] pub retryable: bool, #[serde(default)] pub reason: String, #[serde(default)] pub message: String }\n"
        + "#[derive(Clone, Debug, Default, Eq, PartialEq, Serialize, Deserialize)]\n"
        + "pub struct CollectorError { #[serde(default)] pub code: String, #[serde(default)] pub message: String, #[serde(default)] pub retryable: bool, #[serde(default)] pub field: String, #[serde(default)] pub event_id: String }\n"
        + "#[derive(Clone, Debug, Default, Eq, PartialEq, Serialize, Deserialize)]\n"
        + "pub struct CollectorResponse { #[serde(default)] pub request_id: String, #[serde(default)] pub status: String, #[serde(default)] pub accepted: u64, #[serde(default)] pub rejected: u64, #[serde(default)] pub invalid: u64, #[serde(default)] pub deduped: u64, #[serde(default)] pub reason: String, #[serde(default)] pub error: String, #[serde(default)] pub acks: Vec<CollectorAck>, #[serde(default)] pub errors: Vec<CollectorError> }\n"
    )

