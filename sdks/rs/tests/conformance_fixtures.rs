use serde_json::Value;
use std::fs;
use std::path::PathBuf;

fn repo_root() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("..").join("..")
}

fn spec_root() -> PathBuf {
    repo_root().join("spec")
}

fn manifest_path() -> PathBuf {
    let canonical = spec_root().join("conformance").join("manifest.json");
    if canonical.exists() {
        canonical
    } else {
        spec_root()
            .join("examples")
            .join("golden")
            .join("manifest.json")
    }
}

fn fixture_paths(manifest: &Value, key: &str) -> Vec<PathBuf> {
    let root = manifest_path()
        .parent()
        .expect("manifest dir")
        .to_path_buf();
    manifest[key]
        .as_array()
        .expect("fixture group")
        .iter()
        .map(|value| root.join(value.as_str().expect("fixture path")))
        .collect()
}

fn validate_event(payload: &Value) {
    let obj = payload.as_object().expect("event object");
    assert_eq!(
        obj.get("schema_version").and_then(Value::as_str),
        Some("v1")
    );
    assert_eq!(obj.get("event_version").and_then(Value::as_str), Some("v1"));
    assert!(obj
        .get("event_id")
        .and_then(Value::as_str)
        .is_some_and(|v| !v.trim().is_empty()));
    assert!(obj
        .get("timestamp")
        .and_then(Value::as_str)
        .is_some_and(|v| !v.trim().is_empty()));
    let service = obj.get("service").expect("service");
    match service {
        Value::String(text) => assert!(!text.trim().is_empty()),
        Value::Object(map) => assert!(map
            .get("name")
            .and_then(Value::as_str)
            .is_some_and(|v| !v.trim().is_empty())),
        _ => panic!("service must be string or object"),
    }
    assert!(obj
        .get("event")
        .and_then(Value::as_str)
        .is_some_and(|v| !v.trim().is_empty()));
}

#[test]
fn valid_golden_fixtures_match_expected_contract() {
    let manifest: Value =
        serde_json::from_str(&fs::read_to_string(manifest_path()).expect("manifest"))
            .expect("json");
    for path in fixture_paths(&manifest, "valid") {
        let payload: Value =
            serde_json::from_str(&fs::read_to_string(path).expect("fixture")).expect("json");
        if payload.get("request_id").is_some() && payload.get("status").is_some() {
            assert!(matches!(
                payload["status"].as_str(),
                Some("accepted" | "partial" | "rejected" | "invalid")
            ));
            continue;
        }
        validate_event(&payload);
    }
}

#[test]
fn invalid_golden_fixtures_stay_invalid_by_shape() {
    let manifest: Value =
        serde_json::from_str(&fs::read_to_string(manifest_path()).expect("manifest"))
            .expect("json");
    let invalid = fixture_paths(&manifest, "invalid");
    let missing_event_id_path = invalid
        .iter()
        .find(|path| {
            path.file_name()
                .is_some_and(|name| name == "missing_event_id.json")
        })
        .expect("missing event id fixture");
    let missing_event_id: Value =
        serde_json::from_str(&fs::read_to_string(missing_event_id_path).expect("fixture"))
            .expect("json");
    assert!(missing_event_id.get("event_id").is_none());

    let invalid_enums_path = invalid
        .iter()
        .find(|path| {
            path.file_name()
                .is_some_and(|name| name == "invalid_enum_values.json")
        })
        .expect("invalid enum fixture");
    let invalid_enums: Value =
        serde_json::from_str(&fs::read_to_string(invalid_enums_path).expect("fixture"))
            .expect("json");
    assert_ne!(invalid_enums["kind"].as_str(), Some("event"));

    let missing_versions_path = invalid
        .iter()
        .find(|path| {
            path.file_name()
                .is_some_and(|name| name == "missing_versions.json")
        })
        .expect("missing versions fixture");
    let missing_versions: Value =
        serde_json::from_str(&fs::read_to_string(missing_versions_path).expect("fixture"))
            .expect("json");
    assert!(
        missing_versions.get("schema_version").is_none()
            || missing_versions.get("event_version").is_none()
    );
}
