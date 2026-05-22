use serde_json::Value;
use std::fs;
use std::path::PathBuf;

fn repo_root() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("..")
        .join("..")
}

#[test]
fn loxa_spec_manifest_points_to_existing_contract_files() {
    let root = repo_root();
    let manifest_path = {
        let canonical = root.join("spec/conformance/manifest.json");
        if canonical.exists() {
            canonical
        } else {
            root.join("spec/examples/golden/manifest.json")
        }
    };
    let raw = fs::read_to_string(&manifest_path).expect("read manifest");
    let manifest: Value = serde_json::from_str(&raw).expect("parse manifest");

    for key in ["strict_schema", "loose_schema"] {
        let rel = manifest
            .get(key)
            .and_then(Value::as_str)
            .expect("schema path");
        let path = manifest_path.parent().expect("manifest dir").join(rel);
        assert!(
            path.exists(),
            "expected {key} file to exist at {}",
            path.display()
        );
    }

    for group in [
        "valid",
        "loose_only_valid",
        "invalid",
        "strict_only_invalid",
        "invalid_ingest",
        "invalid_collector_response",
        "invalid_limits",
        "emitted_shape",
        "collector_ack_behavior",
        "ingest_envelopes",
    ] {
        let items = manifest
            .get(group)
            .and_then(Value::as_array)
            .expect("fixture list");
        assert!(!items.is_empty(), "expected fixtures in {group}");
        for item in items {
            let rel = item.as_str().expect("fixture path");
            let path = manifest_path.parent().expect("manifest dir").join(rel);
            assert!(
                path.exists(),
                "expected fixture file to exist at {}",
                path.display()
            );
        }
    }
}
