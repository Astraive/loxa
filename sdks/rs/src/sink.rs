use crate::config::SinkConfig;
use crate::core::client::CollectorHttpClient;
use crate::generated::spec_contract::parse_collector_response_value;
use crate::internal::retry::RetryPolicy;
use flate2::write::GzEncoder;
use flate2::Compression;
use serde_json::Value;
use std::env;
use std::fs::OpenOptions;
use std::io::{self, Write};
use std::sync::{Mutex, OnceLock};
use std::thread;
use std::time::Duration;
use time::format_description::well_known::Rfc2822;
use time::OffsetDateTime;

/// Callback for collector ack/nack responses.
pub type CollectorAckHandler = Box<dyn Fn(&Value) + Send + Sync>;

pub fn write_sink(sink: &SinkConfig, encoded: &str) -> io::Result<()> {
    write_sink_with_ack(sink, encoded, None)
}

pub fn write_sink_with_ack(
    sink: &SinkConfig,
    encoded: &str,
    ack: Option<&CollectorAckHandler>,
) -> io::Result<()> {
    match sink {
        SinkConfig::Stdout => {
            println!("{encoded}");
            Ok(())
        }
        SinkConfig::Stderr => {
            eprintln!("{encoded}");
            Ok(())
        }
        SinkConfig::File(path) => {
            // Global mutex to prevent concurrent file writes from corrupting data
            static FILE_WRITE_LOCK: OnceLock<Mutex<()>> = OnceLock::new();
            let _guard = FILE_WRITE_LOCK.get_or_init(|| Mutex::new(())).lock().unwrap();
            let mut file = OpenOptions::new().create(true).append(true).open(path)?;
            writeln!(file, "{encoded}")
        }
        SinkConfig::Memory(store) => {
            store.events.lock().unwrap().push(encoded.to_string());
            Ok(())
        }
        SinkConfig::Noop => Ok(()),
        SinkConfig::HttpBatch {
            endpoint, ndjson, ..
        } => {
            if *ndjson {
                post_http_ndjson_with_ack(endpoint, &[encoded.to_string()], ack)
            } else {
                post_http_batch_with_ack(endpoint, &[encoded.to_string()], ack)
            }
        }
    }
}

pub fn write_batch_sink(sink: &SinkConfig, encoded_events: &[String]) -> io::Result<()> {
    write_batch_sink_with_ack(sink, encoded_events, None)
}

pub fn write_batch_sink_with_ack(
    sink: &SinkConfig,
    encoded_events: &[String],
    ack: Option<&CollectorAckHandler>,
) -> io::Result<()> {
    match sink {
        SinkConfig::HttpBatch {
            endpoint, ndjson, ..
        } => {
            if *ndjson {
                post_http_ndjson_with_ack(endpoint, encoded_events, ack)
            } else {
                post_http_batch_with_ack(endpoint, encoded_events, ack)
            }
        }
        _ => {
            for encoded in encoded_events {
                write_sink_with_ack(sink, encoded, ack)?;
            }
            Ok(())
        }
    }
}

pub fn flush_sink(sink: &SinkConfig) -> io::Result<()> {
    match sink {
        SinkConfig::Stdout => io::stdout().flush(),
        SinkConfig::Stderr => io::stderr().flush(),
        SinkConfig::File(_)
        | SinkConfig::Memory(_)
        | SinkConfig::Noop
        | SinkConfig::HttpBatch { .. } => Ok(()),
    }
}

pub fn close_sink(sink: &SinkConfig) -> io::Result<()> {
    flush_sink(sink)
}

#[allow(dead_code)]
fn post_http_batch(endpoint: &str, encoded_events: &[String]) -> io::Result<()> {
    post_http_batch_with_ack(endpoint, encoded_events, None)
}

fn post_http_batch_with_ack(
    endpoint: &str,
    encoded_events: &[String],
    ack: Option<&CollectorAckHandler>,
) -> io::Result<()> {
    let client = collector_http_client(endpoint);
    let payload = client.envelope(encoded_events);
    client
        .validate_envelope(&payload)
        .map_err(|err| io::Error::other(format!("collector envelope validation failed: {err}")))?;
    let body = serde_json::to_vec(&payload)
        .map_err(|err| io::Error::other(format!("serialize collector payload: {err}")))?;

    let mut encoder = GzEncoder::new(Vec::new(), Compression::default());
    encoder.write_all(&body)?;
    let compressed = encoder.finish()?;

    let mut request = ureq::post(endpoint)
        .set("Content-Type", "application/json")
        .set("Content-Encoding", "gzip")
        .timeout(std::time::Duration::from_millis(client.timeout_ms));

    if let Some(api_key) = &client.api_key {
        let auth_value = if client.auth_header.eq_ignore_ascii_case("authorization") {
            format!("Bearer {}", api_key)
        } else {
            api_key.clone()
        };
        request = request.set(&client.auth_header, &auth_value);
    }
    let retry_policy = RetryPolicy {
        max_attempts: 3,
        base_delay: Duration::from_millis(100),
        max_delay: Duration::from_secs(30),
    };

    for attempt in 1..=retry_policy.max_attempts {
        let request = request.clone();
        match request.send_bytes(&compressed) {
            Ok(response) => {
                let status = response.status();
                let retry_after = parse_retry_after(response.header("Retry-After"));
                let raw = response.into_string().map_err(|err| {
                    io::Error::other(format!("collector response read failed: {err}"))
                })?;
                notify_ack(ack, &raw);
                match classify_collector_response(status, &raw) {
                    Ok(()) => return Ok(()),
                    Err(CollectorOutcome::Retryable(_detail))
                        if attempt < retry_policy.max_attempts =>
                    {
                        sleep_before_retry(&retry_policy, attempt, retry_after);
                        continue;
                    }
                    Err(CollectorOutcome::Retryable(detail))
                    | Err(CollectorOutcome::Permanent(detail)) => {
                        return Err(io::Error::other(format!(
                            "collector rejected batch: status={status} {detail}"
                        )));
                    }
                }
            }
            Err(ureq::Error::Status(status, response)) => {
                let retry_after = parse_retry_after(response.header("Retry-After"));
                let raw = response.into_string().map_err(|err| {
                    io::Error::other(format!("collector response read failed: {err}"))
                })?;
                notify_ack(ack, &raw);
                match classify_collector_response(status, &raw) {
                    Ok(()) => return Ok(()),
                    Err(CollectorOutcome::Retryable(_detail))
                        if attempt < retry_policy.max_attempts =>
                    {
                        sleep_before_retry(&retry_policy, attempt, retry_after);
                        continue;
                    }
                    Err(CollectorOutcome::Retryable(detail))
                    | Err(CollectorOutcome::Permanent(detail)) => {
                        return Err(io::Error::other(format!(
                            "collector rejected batch: status={status} {detail}"
                        )));
                    }
                }
            }
            Err(ureq::Error::Transport(_err)) if attempt < retry_policy.max_attempts => {
                sleep_before_retry(&retry_policy, attempt, None);
                continue;
            }
            Err(ureq::Error::Transport(err)) => {
                return Err(io::Error::other(format!("collector request failed: {err}")));
            }
        }
    }

    Ok(())
}

fn notify_ack(ack: Option<&CollectorAckHandler>, raw: &str) {
    if let Some(handler) = ack {
        if let Ok(parsed) = serde_json::from_str::<Value>(raw) {
            handler(&parsed);
        }
    }
}

#[allow(dead_code)]
fn post_http_ndjson(endpoint: &str, encoded_events: &[String]) -> io::Result<()> {
    post_http_ndjson_with_ack(endpoint, encoded_events, None)
}

fn post_http_ndjson_with_ack(
    endpoint: &str,
    encoded_events: &[String],
    ack: Option<&CollectorAckHandler>,
) -> io::Result<()> {
    let client = collector_http_client(endpoint);
    let body = encoded_events.join("\n");
    let body_bytes = body.into_bytes();

    let mut encoder = GzEncoder::new(Vec::new(), Compression::default());
    encoder.write_all(&body_bytes)?;
    let compressed = encoder.finish()?;

    let mut request = ureq::post(endpoint)
        .set("Content-Type", "application/x-ndjson")
        .set("Content-Encoding", "gzip")
        .timeout(std::time::Duration::from_millis(client.timeout_ms));

    if let Some(api_key) = &client.api_key {
        let auth_value = if client.auth_header.eq_ignore_ascii_case("authorization") {
            format!("Bearer {}", api_key)
        } else {
            api_key.clone()
        };
        request = request.set(&client.auth_header, &auth_value);
    }
    let retry_policy = RetryPolicy {
        max_attempts: 3,
        base_delay: Duration::from_millis(100),
        max_delay: Duration::from_secs(30),
    };

    for attempt in 1..=retry_policy.max_attempts {
        let request = request.clone();
        match request.send_bytes(&compressed) {
            Ok(response) => {
                let status = response.status();
                let retry_after = parse_retry_after(response.header("Retry-After"));
                let raw = response.into_string().map_err(|err| {
                    io::Error::other(format!("collector response read failed: {err}"))
                })?;
                notify_ack(ack, &raw);
                match classify_collector_response(status, &raw) {
                    Ok(()) => return Ok(()),
                    Err(CollectorOutcome::Retryable(_detail))
                        if attempt < retry_policy.max_attempts =>
                    {
                        sleep_before_retry(&retry_policy, attempt, retry_after);
                        continue;
                    }
                    Err(CollectorOutcome::Retryable(detail))
                    | Err(CollectorOutcome::Permanent(detail)) => {
                        return Err(io::Error::other(format!(
                            "collector rejected ndjson: status={status} {detail}"
                        )));
                    }
                }
            }
            Err(ureq::Error::Status(status, response)) => {
                let retry_after = parse_retry_after(response.header("Retry-After"));
                let raw = response.into_string().map_err(|err| {
                    io::Error::other(format!("collector response read failed: {err}"))
                })?;
                notify_ack(ack, &raw);
                match classify_collector_response(status, &raw) {
                    Ok(()) => return Ok(()),
                    Err(CollectorOutcome::Retryable(_detail))
                        if attempt < retry_policy.max_attempts =>
                    {
                        sleep_before_retry(&retry_policy, attempt, retry_after);
                        continue;
                    }
                    Err(CollectorOutcome::Retryable(detail))
                    | Err(CollectorOutcome::Permanent(detail)) => {
                        return Err(io::Error::other(format!(
                            "collector rejected ndjson: status={status} {detail}"
                        )));
                    }
                }
            }
            Err(ureq::Error::Transport(_err)) if attempt < retry_policy.max_attempts => {
                sleep_before_retry(&retry_policy, attempt, None);
                continue;
            }
            Err(ureq::Error::Transport(err)) => {
                return Err(io::Error::other(format!("collector request failed: {err}")));
            }
        }
    }

    Ok(())
}

enum CollectorOutcome {
    Retryable(String),
    Permanent(String),
}

fn classify_collector_response(status: u16, raw: &str) -> Result<(), CollectorOutcome> {
    let parsed_value: Value = serde_json::from_str(raw).map_err(|err| {
        CollectorOutcome::Permanent(format!("collector response decode failed: {err}"))
    })?;
    let ack = parse_collector_response_value(&parsed_value).map_err(|err| {
        CollectorOutcome::Permanent(format!("collector response decode failed: {err}"))
    })?;

    if let Some(detail) = ack
        .retryable_error()
        .or_else(|| retryable_error_message(&parsed_value))
    {
        return Err(CollectorOutcome::Retryable(detail));
    }
    if status == 429 || status == 503 {
        return Err(CollectorOutcome::Retryable(format!(
            "accepted={} rejected={} invalid={}",
            ack.accepted, ack.rejected, ack.invalid
        )));
    }
    if status >= 300 || ack.rejected > 0 || ack.invalid > 0 {
        let detail = ack.permanent_failure().unwrap_or_else(|| {
            format!(
                "accepted={} rejected={} invalid={}",
                ack.accepted, ack.rejected, ack.invalid
            )
        });
        return Err(CollectorOutcome::Permanent(detail));
    }

    Ok(())
}

fn sleep_before_retry(policy: &RetryPolicy, attempt: u32, retry_after: Option<Duration>) {
    let delay = retry_after.unwrap_or_else(|| policy.delay(attempt));
    thread::sleep(delay.min(policy.max_delay));
}

fn parse_retry_after(value: Option<&str>) -> Option<Duration> {
    let raw = value?.trim();
    if raw.is_empty() {
        return None;
    }
    if let Ok(seconds) = raw.parse::<u64>() {
        return Some(Duration::from_secs(seconds));
    }
    let parsed = OffsetDateTime::parse(raw, &Rfc2822).ok()?;
    let now = OffsetDateTime::now_utc();
    if parsed <= now {
        return None;
    }
    Some(Duration::from_secs((parsed - now).whole_seconds() as u64))
}

fn retryable_error_message(value: &Value) -> Option<String> {
    value
        .get("errors")
        .and_then(Value::as_array)
        .and_then(|items| {
            items.iter().find_map(|item| {
                if item.get("retryable").and_then(Value::as_bool) == Some(true) {
                    return item
                        .get("message")
                        .and_then(Value::as_str)
                        .map(ToString::to_string)
                        .or_else(|| {
                            item.get("code")
                                .and_then(Value::as_str)
                                .map(ToString::to_string)
                        });
                }
                None
            })
        })
}

fn collector_http_client(endpoint: &str) -> CollectorHttpClient {
    let mut client = CollectorHttpClient::new(endpoint.to_string());
    if let Ok(api_key) = env::var("LOXA_COLLECTOR_API_KEY") {
        let api_key = api_key.trim();
        if !api_key.is_empty() {
            client = client.with_api_key(api_key.to_string());
        }
    }
    if let Ok(header) = env::var("LOXA_COLLECTOR_API_KEY_HEADER") {
        let header = header.trim();
        if !header.is_empty() {
            client.auth_header = header.to_string();
        }
    }
    client
}
