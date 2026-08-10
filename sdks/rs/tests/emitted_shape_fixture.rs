use loza::{Config, New, Params, SinkConfig};
use serde::Deserialize;
use serde_json::Value;
use std::collections::BTreeMap;
use std::fs;
use std::path::PathBuf;

#[derive(Debug, Deserialize)]
struct Fixture {
    params: FixtureParams,
    attrs: BTreeMap<String, Value>,
    finish: FixtureFinish,
    expected: FixtureExpected,
}

#[derive(Debug, Deserialize)]
struct FixtureParams {
    service: String,
    event: String,
    kind: String,
    method: String,
    path: String,
    route: String,
    status_code: u16,
}

#[derive(Debug, Deserialize)]
struct FixtureFinish {
    outcome: String,
}

#[derive(Debug, Deserialize)]
struct FixtureExpected {
    present: Vec<String>,
    equals: BTreeMap<String, Value>,
}

#[test]
fn shared_emitted_shape_fixture() {
    let fixture = load_fixture();
    let logger = New(Config::test(&fixture.params.service).with_sink(SinkConfig::Noop));
    let mut ctx = logger.start_event(
        Params::new(&fixture.params.event)
            .with_kind(&fixture.params.kind)
            .with_method(&fixture.params.method)
            .with_path(&fixture.params.path)
            .with_route(&fixture.params.route)
            .with_status_code(fixture.params.status_code),
    );
    for (key, value) in &fixture.attrs {
        logger.set(&mut ctx, key, value.clone());
    }
    logger
        .finish(&mut ctx, &fixture.finish.outcome)
        .expect("finish event");
    let encoded = logger.emit(&ctx).expect("emit event");
    let payload: Value = serde_json::from_str(&encoded).expect("decode payload");

    for path in &fixture.expected.present {
        assert!(
            lookup_path(&payload, path).is_some(),
            "expected {path} to be present"
        );
    }
    for (path, want) in &fixture.expected.equals {
        let got = lookup_path(&payload, path).unwrap_or_else(|| panic!("missing {path}"));
        assert_eq!(got, want, "unexpected value for {path}");
    }
}

fn load_fixture() -> Fixture {
    let path = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("..")
        .join("..")
        .join("spec")
        .join("examples")
        .join("golden")
        .join("emitted-shape")
        .join("structured_http_success.json");
    serde_json::from_str(&fs::read_to_string(path).expect("fixture")).expect("fixture json")
}

fn lookup_path<'a>(value: &'a Value, path: &str) -> Option<&'a Value> {
    let mut current = value;
    for segment in path.split('.') {
        current = current.get(segment)?;
    }
    Some(current)
}
