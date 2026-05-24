use loxa::core::client::CollectorHttpClient;
use serde::Deserialize;
use serde_json::Value;
use std::fs;
use std::path::PathBuf;

#[derive(Debug, Deserialize)]
struct Fixture {
    input_events: Vec<Value>,
    expected: Expected,
}

#[derive(Debug, Deserialize)]
struct Expected {
    #[serde(rename = "api_version")]
    api_version: String,
    #[serde(rename = "source.service")]
    source_service: String,
    #[serde(rename = "events_count")]
    events_count: usize,
    #[serde(rename = "first_event.event")]
    first_event_event: String,
    #[serde(rename = "first_event.service")]
    first_event_service: String,
}

fn repo_root() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("..")
        .join("..")
}

fn first_existing(paths: &[PathBuf]) -> PathBuf {
    for path in paths {
        if path.exists() {
            return path.clone();
        }
    }
    paths[0].clone()
}

#[test]
fn collector_http_client_matches_wrapped_batch_envelope_fixture() {
    let root = repo_root();
    let path = first_existing(&[
        root.join("spec/fixtures/ingest/wrapped_batch_json.json"),
        root.join("spec/examples/golden/ingest-envelopes/wrapped_batch_json.json"),
    ]);
    let raw = fs::read_to_string(path).expect("read fixture");
    let fixture: Fixture = serde_json::from_str(&raw).expect("parse fixture");
    let encoded: Vec<String> = fixture
        .input_events
        .iter()
        .map(|value| serde_json::to_string(value).expect("encode input event"))
        .collect();

    let mut client = CollectorHttpClient::new("http://collector.example/events");
    client.service = Some("checkout".to_string());
    let envelope = client.envelope(&encoded);
    client
        .validate_envelope(&envelope)
        .expect("validate envelope");

    assert_eq!(envelope["api_version"], fixture.expected.api_version);
    assert_eq!(
        envelope["source"]["service"],
        fixture.expected.source_service
    );
    let events = envelope["events"].as_array().expect("events array");
    assert_eq!(events.len(), fixture.expected.events_count);
    assert_eq!(
        envelope["events"][0]["event"],
        fixture.expected.first_event_event
    );
    assert_eq!(
        envelope["events"][0]["service"],
        fixture.expected.first_event_service
    );
}

#[test]
fn collector_http_client_rejects_invalid_envelope() {
    let client = CollectorHttpClient::new("http://collector.example/events");
    let invalid = serde_json::json!({
        "api_version": "v1",
        "source": { "sdk": "loxa-rs", "version": "0.0.2", "service": "checkout" },
        "events": []
    });
    assert!(client.validate_envelope(&invalid).is_err());
}
