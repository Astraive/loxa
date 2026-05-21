use serde_json::Value;
use std::fs;
use std::path::PathBuf;

const REQUIRED_DIRS: &[&str] = &[
    ".github/workflows",
    "bench",
    "docs/rules",
    "examples/basic",
    "examples/custom-schema",
    "examples/httpbatch-to-collector",
    "src/config",
    "src/core",
    "src/cortex",
    "src/generated",
    "src/integrations/log",
    "src/integrations/otel",
    "src/integrations/tracing",
    "src/internal/core",
    "src/internal/clock",
    "src/internal/env",
    "src/internal/jsonenc",
    "src/internal/pool",
    "src/internal/queue",
    "src/internal/retry",
    "src/internal/safe",
    "src/internal/transport",
    "src/middleware/actix",
    "src/middleware/axum",
    "src/middleware/hyper",
    "src/middleware/tower",
    "src/redaction",
    "src/sinks/httpbatch",
    "src/storagepath",
    "src/testkit",
    "src/utils",
    "tests/conformance",
    "tests/integration",
    "tests/root",
];

#[test]
fn required_repo_tree_exists() {
    let root = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    let missing: Vec<_> = REQUIRED_DIRS
        .iter()
        .filter(|dir| !root.join(dir).is_dir())
        .copied()
        .collect();
    assert!(missing.is_empty(), "missing required dirs: {missing:?}");
}

#[test]
fn superset_manifest_is_readable_for_parity_gate() {
    let root = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    let manifest_path = root
        .parent()
        .expect("workspace parent")
        .join("loxa-spec")
        .join("docs")
        .join("sdk-parity-manifest.json");
    let manifest: Value =
        serde_json::from_str(&fs::read_to_string(manifest_path).expect("manifest")).expect("json");
    assert_eq!(manifest["scope"], "lightweight-sdk");
    assert!(manifest["excluded_from_sdk"]
        .as_array()
        .unwrap()
        .iter()
        .any(|v| v == "Kafka"));
}

#[test]
fn public_api_names_match_superset_manifest() {
    let root = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    let manifest_path = root
        .parent()
        .expect("workspace parent")
        .join("loxa-spec")
        .join("docs")
        .join("sdk-parity-manifest.json");
    let manifest: Value =
        serde_json::from_str(&fs::read_to_string(manifest_path).expect("manifest")).expect("json");
    let source = fs::read_to_string(root.join("src").join("lib.rs")).expect("lib source");
    let mut missing = Vec::new();

    for (key, values) in manifest.as_object().expect("manifest object") {
        if key == "excluded_from_sdk" || key == "sdks" {
            continue;
        }
        let Some(values) = values.as_array() else {
            continue;
        };
        for value in values {
            let name = value.as_str().expect("api name");
            if !source.contains(name) {
                missing.push(name.to_string());
            }
        }
    }

    assert!(missing.is_empty(), "missing public API names: {missing:?}");
}
