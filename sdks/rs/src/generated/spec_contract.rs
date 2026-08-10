#![allow(dead_code)]

use serde::{Deserialize, Serialize};
use serde_json::{json, Map, Value};
use std::collections::BTreeSet;

pub const LOZA_SPEC_VERSION: &str = "v1";
pub const LOZA_INGEST_API_VERSION: &str = "v1";
pub const LOZA_EVENT_VERSION: &str = "v1";
pub const MAX_EVENT_BYTES: usize = 65536;
pub const MAX_BATCH_EVENTS: usize = 1000;
pub const MAX_BATCH_SIZE_BYTES: usize = 1048576;
pub const MAX_ATTRS_DEPTH: usize = 8;
pub const MAX_ATTR_KEY_LENGTH: usize = 128;
pub const MAX_ATTR_VALUE_LENGTH: usize = 4096;
pub const MAX_ERROR_STACK_LENGTH: usize = 16384;

pub const ALLOWED_KINDS: &[&str] = &[
    "event",
    "http",
    "job",
    "queue",
    "cli",
    "cron",
    "log",
    "checkpoint",
];
pub const ALLOWED_LEVELS: &[&str] = &["debug", "info", "warn", "error", "fatal"];
pub const ALLOWED_OUTCOMES: &[&str] = &[
    "success",
    "error",
    "partial",
    "abandoned",
    "retried",
    "cancelled",
    "timeout",
    "skipped",
    "rejected",
    "quarantined",
    "unknown",
];
pub const ALLOWED_PARTIAL_REASONS: &[&str] = &[
    "not_finished",
    "process_exit",
    "timeout",
    "panic",
    "collector_unavailable",
];
pub const ALLOWED_EVENT_STATES: &[&str] = &[
    "created",
    "active",
    "finished",
    "emitting",
    "emitted",
    "invalid",
    "dropped",
    "emit_failed",
    "spooled",
    "dlq_written",
    "failed_validation",
    "delivery_failed",
];
pub const ALLOWED_COLLECTOR_STATUSES: &[&str] = &["accepted", "partial", "rejected", "invalid"];

pub const ALLOWED_TOP_LEVEL_FIELDS: &[&str] = &[
    "attrs",
    "checkpoints",
    "collector",
    "delivery_attempts",
    "deployment",
    "duration_ms",
    "environment",
    "error",
    "errors",
    "event",
    "event_id",
    "event_state",
    "event_version",
    "groups",
    "http",
    "kind",
    "level",
    "links",
    "message",
    "method",
    "organization",
    "outcome",
    "partial",
    "partial_reason",
    "path",
    "pii",
    "processes",
    "redaction",
    "request_id",
    "resource",
    "region",
    "route",
    "sampling",
    "schema_version",
    "sdk",
    "service",
    "source",
    "span_id",
    "incident_id",
    "parent_id",
    "status_code",
    "tenant",
    "timestamp",
    "timers",
    "trace_flags",
    "trace_id",
    "user",
    "version",
    "workspace",
];

pub const CANONICAL_FIELDS: &[&str] = &[
    "timestamp",
    "schema_version",
    "event_version",
    "event_id",
    "request_id",
    "trace_id",
    "span_id",
    "parent_id",
    "incident_id",
    "level",
    "event",
    "kind",
    "message",
    "outcome",
    "duration_ms",
    "service",
    "version",
    "environment",
    "deployment_id",
    "region",
    "host",
    "runtime",
    "method",
    "path",
    "route",
    "status_code",
    "error",
    "checkpoints",
    "processes",
    "groups",
    "timers",
    "links",
    "partial",
    "partial_reason",
    "event_state",
    "delivery_attempts",
    "http",
    "user",
    "tenant",
    "workspace",
    "organization",
    "pii",
    "resource",
    "attrs",
    "sdk",
    "collector",
    "sampling",
    "redaction",
    "deployment",
    "source",
];

#[derive(Clone, Debug, Default, Eq, PartialEq, Serialize, Deserialize)]
pub struct CollectorAck {
    #[serde(default)]
    pub event_id: String,
    #[serde(default)]
    pub status: String,
    #[serde(default)]
    pub retryable: bool,
    #[serde(default)]
    pub reason: String,
    #[serde(default)]
    pub message: String,
}

#[derive(Clone, Debug, Default, Eq, PartialEq, Serialize, Deserialize)]
pub struct CollectorError {
    #[serde(default)]
    pub code: String,
    #[serde(default)]
    pub message: String,
    #[serde(default)]
    pub retryable: bool,
}

#[derive(Clone, Debug, Default, Eq, PartialEq, Serialize, Deserialize)]
pub struct CollectorResponse {
    #[serde(default)]
    pub request_id: String,
    #[serde(default)]
    pub status: String,
    #[serde(default)]
    pub accepted: u64,
    #[serde(default)]
    pub rejected: u64,
    #[serde(default)]
    pub invalid: u64,
    #[serde(default)]
    pub deduped: u64,
    #[serde(default)]
    pub reason: String,
    #[serde(default)]
    pub error: String,
    #[serde(default)]
    pub acks: Vec<CollectorAck>,
    #[serde(default)]
    pub errors: Vec<CollectorError>,
}

impl CollectorResponse {
    pub fn retryable_error(&self) -> Option<String> {
        self.errors
            .iter()
            .find(|item| item.retryable)
            .map(|item| {
                if !item.message.is_empty() {
                    item.message.clone()
                } else {
                    item.code.clone()
                }
            })
            .or_else(|| {
                self.acks.iter().find(|ack| ack.retryable).map(|ack| {
                    if !ack.message.is_empty() {
                        ack.message.clone()
                    } else if !ack.reason.is_empty() {
                        ack.reason.clone()
                    } else {
                        ack.status.clone()
                    }
                })
            })
    }

    pub fn permanent_failure(&self) -> Option<String> {
        if self.rejected == 0
            && self.invalid == 0
            && !matches!(self.status.as_str(), "partial" | "rejected" | "invalid")
        {
            return None;
        }
        if let Some(ack) = self
            .acks
            .iter()
            .find(|ack| !ack.retryable && matches!(ack.status.as_str(), "rejected" | "invalid"))
        {
            return Some(if !ack.reason.is_empty() {
                ack.reason.clone()
            } else if !ack.message.is_empty() {
                ack.message.clone()
            } else {
                ack.status.clone()
            });
        }
        if let Some(item) = self
            .errors
            .iter()
            .find(|item| !item.retryable && (!item.message.is_empty() || !item.code.is_empty()))
        {
            return Some(if !item.message.is_empty() {
                item.message.clone()
            } else {
                item.code.clone()
            });
        }
        Some(if !self.error.is_empty() {
            self.error.clone()
        } else if !self.reason.is_empty() {
            self.reason.clone()
        } else {
            format!(
                "accepted={} rejected={} invalid={}",
                self.accepted, self.rejected, self.invalid
            )
        })
    }
}

pub fn is_canonical(key: &str) -> bool {
    CANONICAL_FIELDS.contains(&key)
}

pub fn looks_like_loza_event(payload: &Map<String, Value>) -> bool {
    payload.contains_key("schema_version")
        || payload.contains_key("event_version")
        || payload.contains_key("event")
        || payload.contains_key("event_type")
}

pub fn normalize_event_aliases_copy(payload: &Map<String, Value>) -> (Map<String, Value>, bool) {
    let mut normalized = payload.clone();
    if normalized
        .get("event")
        .and_then(Value::as_str)
        .map(|value| !value.trim().is_empty())
        == Some(true)
    {
        let changed = normalized.remove("event_type").is_some();
        return (normalized, changed);
    }
    if let Some(alias) = normalized
        .get("event_type")
        .and_then(Value::as_str)
        .map(str::trim)
        .filter(|value| !value.is_empty())
    {
        normalized.insert("event".to_string(), Value::String(alias.to_string()));
        normalized.remove("event_type");
        return (normalized, true);
    }
    (normalized, false)
}

pub fn normalize_event_aliases(payload: &mut Map<String, Value>) -> bool {
    let (normalized, changed) = normalize_event_aliases_copy(payload);
    *payload = normalized;
    changed
}

pub fn build_ingest_envelope(
    events: &[Value],
    sdk_name: &str,
    sdk_version: &str,
    service: &str,
) -> Value {
    let normalized_events: Vec<Value> = events
        .iter()
        .map(|event| {
            if let Some(obj) = event.as_object() {
                let (normalized, _) = normalize_event_aliases_copy(obj);
                Value::Object(normalized)
            } else {
                event.clone()
            }
        })
        .collect();
    json!({
        "api_version": LOZA_INGEST_API_VERSION,
        "source": {
            "sdk": sdk_name,
            "version": sdk_version,
            "service": if service.trim().is_empty() { infer_service(&normalized_events) } else { service.trim().to_string() },
        },
        "events": normalized_events,
    })
}

pub fn parse_collector_response_value(
    value: &Value,
) -> Result<CollectorResponse, serde_json::Error> {
    serde_json::from_value(value.clone())
}

pub fn validate_event_value(
    value: &Value,
    strict: bool,
) -> Result<(), crate::errors::ValidationError> {
    let object = value.as_object().ok_or_else(|| {
        crate::errors::ValidationError::new(
            None,
            "invalid_type",
            "event payload must be a JSON object".to_string(),
        )
    })?;
    let (normalized, _) = normalize_event_aliases_copy(object);
    if strict {
        let allowed: BTreeSet<&str> = [
            "event_id",
            "timestamp",
            "schema_version",
            "event_version",
            "service",
            "version",
            "environment",
            "deployment",
            "source",
            "event",
            "kind",
            "message",
            "level",
            "outcome",
            "duration_ms",
            "partial",
            "partial_reason",
            "event_state",
            "delivery_attempts",
            "checkpoints",
            "request_id",
            "trace_id",
            "span_id",
            "parent_id",
            "incident_id",
            "method",
            "path",
            "route",
            "status_code",
            "http",
            "error",
            "user",
            "tenant",
            "workspace",
            "organization",
            "pii",
            "resource",
            "attrs",
            "region",
        ]
        .into_iter()
        .collect();
        for key in normalized.keys() {
            if !allowed.contains(key.as_str()) {
                return Err(crate::errors::ValidationError::new(
                    Some(key.as_str()),
                    "not_allowed_by_strict_schema",
                    format!("field \"{}\" is not allowed by strict schema", key),
                ));
            }
        }
    }
    require_enum(&normalized, "schema_version", &[LOZA_SPEC_VERSION])?;
    require_enum(&normalized, "event_version", &[LOZA_EVENT_VERSION])?;
    require_non_empty_string(&normalized, "event_id")?;
    require_rfc3339(&normalized, "timestamp")?;
    require_service(&normalized)?;
    require_non_empty_string(&normalized, "event")?;
    require_enum(
        &normalized,
        "kind",
        &[
            "event",
            "http",
            "job",
            "queue",
            "cli",
            "cron",
            "log",
            "checkpoint",
        ],
    )?;
    optional_enum(
        &normalized,
        "level",
        &["debug", "info", "warn", "error", "fatal"],
    )?;
    optional_enum(
        &normalized,
        "outcome",
        &[
            "success",
            "error",
            "timeout",
            "cancelled",
            "rejected",
            "abandoned",
            "partial",
            "unknown",
        ],
    )?;
    optional_enum(
        &normalized,
        "partial_reason",
        &[
            "not_finished",
            "process_exit",
            "timeout",
            "panic",
            "collector_unavailable",
        ],
    )?;
    optional_enum(
        &normalized,
        "event_state",
        &[
            "created",
            "active",
            "finished",
            "emitting",
            "emitted",
            "failed_validation",
            "delivery_failed",
        ],
    )?;
    optional_non_negative_integer(&normalized, "duration_ms")?;
    optional_status_code(&normalized, "status_code")?;
    for key in [
        "http",
        "error",
        "user",
        "tenant",
        "resource",
        "deployment",
        "source",
        "workspace",
        "organization",
        "attrs",
        "pii",
    ] {
        optional_object(&normalized, key)?;
    }
    if let Some(http) = normalized.get("http").and_then(Value::as_object) {
        optional_status_code(http, "status_code")?;
    }
    Ok(())
}

fn infer_service(events: &[Value]) -> String {
    events
        .iter()
        .find_map(|event| event.get("service").and_then(Value::as_str))
        .filter(|value| !value.trim().is_empty())
        .unwrap_or("unknown")
        .to_string()
}

fn require_non_empty_string(
    object: &Map<String, Value>,
    key: &str,
) -> Result<(), crate::errors::ValidationError> {
    match object.get(key).and_then(Value::as_str).map(str::trim) {
        Some(value) if !value.is_empty() => Ok(()),
        _ => Err(crate::errors::ValidationError::new(
            Some(key),
            "required",
            format!("field \"{}\" must be a non-empty string", key),
        )),
    }
}

fn require_enum(
    object: &Map<String, Value>,
    key: &str,
    allowed: &[&str],
) -> Result<(), crate::errors::ValidationError> {
    require_non_empty_string(object, key)?;
    let value = object
        .get(key)
        .and_then(Value::as_str)
        .unwrap_or_default()
        .trim();
    if allowed.contains(&value) {
        Ok(())
    } else {
        Err(crate::errors::ValidationError::new(
            Some(key),
            "unsupported_value",
            format!("field \"{}\" has unsupported value \"{}\"", key, value),
        ))
    }
}

fn optional_enum(
    object: &Map<String, Value>,
    key: &str,
    allowed: &[&str],
) -> Result<(), crate::errors::ValidationError> {
    if !object.contains_key(key) {
        return Ok(());
    }
    let value = object
        .get(key)
        .and_then(Value::as_str)
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .ok_or_else(|| {
            crate::errors::ValidationError::new(
                Some(key),
                "invalid_value",
                format!("field \"{}\" must be a non-empty string when set", key),
            )
        })?;
    if allowed.contains(&value) {
        Ok(())
    } else {
        Err(crate::errors::ValidationError::new(
            Some(key),
            "unsupported_value",
            format!("field \"{}\" has unsupported value \"{}\"", key, value),
        ))
    }
}

fn optional_non_negative_integer(
    object: &Map<String, Value>,
    key: &str,
) -> Result<(), crate::errors::ValidationError> {
    if !object.contains_key(key) {
        return Ok(());
    }
    let value = object.get(key).ok_or_else(|| {
        crate::errors::ValidationError::new(
            Some(key),
            "invalid_integer",
            format!("field \"{}\" must be a non-negative integer", key),
        )
    })?;
    if let Some(number) = value.as_i64() {
        if number >= 0 {
            return Ok(());
        }
    }
    if let Some(number) = value.as_u64() {
        let _ = number;
        return Ok(());
    }
    Err(crate::errors::ValidationError::new(
        Some(key),
        "invalid_integer",
        format!("field \"{}\" must be a non-negative integer", key),
    ))
}

fn optional_status_code(
    object: &Map<String, Value>,
    key: &str,
) -> Result<(), crate::errors::ValidationError> {
    if !object.contains_key(key) {
        return Ok(());
    }
    let value = object.get(key).ok_or_else(|| {
        crate::errors::ValidationError::new(
            Some(key),
            "invalid_status_code",
            format!("field \"{}\" must be between 100 and 599", key),
        )
    })?;
    let code = value.as_i64().ok_or_else(|| {
        crate::errors::ValidationError::new(
            Some(key),
            "invalid_status_code",
            format!("field \"{}\" must be between 100 and 599", key),
        )
    })?;
    if (100..=599).contains(&code) {
        Ok(())
    } else {
        Err(crate::errors::ValidationError::new(
            Some(key),
            "invalid_status_code",
            format!("field \"{}\" must be between 100 and 599", key),
        ))
    }
}

fn optional_object(
    object: &Map<String, Value>,
    key: &str,
) -> Result<(), crate::errors::ValidationError> {
    if !object.contains_key(key) {
        return Ok(());
    }
    if object.get(key).and_then(Value::as_object).is_some() {
        Ok(())
    } else {
        Err(crate::errors::ValidationError::new(
            Some(key),
            "invalid_type",
            format!("field \"{}\" must be an object", key),
        ))
    }
}

fn require_service(object: &Map<String, Value>) -> Result<(), crate::errors::ValidationError> {
    if let Some(value) = object.get("service") {
        if let Some(text) = value.as_str() {
            if !text.trim().is_empty() {
                return Ok(());
            }
        }
        if let Some(service) = value.as_object() {
            if service
                .get("name")
                .and_then(Value::as_str)
                .map(str::trim)
                .filter(|value| !value.is_empty())
                .is_some()
            {
                return Ok(());
            }
        }
    }
    Err(crate::errors::ValidationError::new(
        Some("service"),
        "required",
        "field \"service\" must be a non-empty string or object with name".to_string(),
    ))
}

fn require_rfc3339(
    object: &Map<String, Value>,
    key: &str,
) -> Result<(), crate::errors::ValidationError> {
    let value = object
        .get(key)
        .and_then(Value::as_str)
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .ok_or_else(|| {
            crate::errors::ValidationError::new(
                Some(key),
                "invalid_timestamp",
                format!("field \"{}\" must be RFC3339", key),
            )
        })?;
    time::OffsetDateTime::parse(value, &time::format_description::well_known::Rfc3339)
        .map(|_| ())
        .map_err(|_| {
            crate::errors::ValidationError::new(
                Some(key),
                "invalid_timestamp",
                format!("field \"{}\" must be RFC3339", key),
            )
        })
}
