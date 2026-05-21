use crate::{config::SchemaConfig, EventContext};
use serde_json::{Map, Value};

pub trait Schema {
    fn encode(&self, event: &EventContext) -> Value;
}

pub type SchemaFunc = fn(&EventContext) -> Value;

/// Read-only view of an EventContext, intended for use by custom schemas.
pub struct EventView<'a> {
    inner: &'a EventContext,
}

impl<'a> EventView<'a> {
    pub fn new(event: &'a EventContext) -> Self {
        Self { inner: event }
    }

    pub fn event_id(&self) -> &str {
        &self.inner.event_id
    }
    pub fn service(&self) -> &str {
        &self.inner.service
    }
    pub fn event(&self) -> &str {
        &self.inner.event
    }
    pub fn kind(&self) -> &str {
        &self.inner.kind
    }
    pub fn level(&self) -> &str {
        &self.inner.level
    }
    pub fn outcome(&self) -> Option<&str> {
        self.inner.outcome.as_deref()
    }
    pub fn message(&self) -> Option<&str> {
        self.inner.message.as_deref()
    }
    pub fn duration_ms(&self) -> Option<u128> {
        self.inner.duration_ms
    }
    pub fn timestamp(&self) -> &str {
        &self.inner.timestamp
    }
    pub fn trace_id(&self) -> Option<&str> {
        self.inner.trace_id.as_deref()
    }
    pub fn span_id(&self) -> Option<&str> {
        self.inner.span_id.as_deref()
    }
    pub fn request_id(&self) -> &str {
        &self.inner.request_id
    }
    pub fn method(&self) -> Option<&str> {
        self.inner.method.as_deref()
    }
    pub fn path(&self) -> Option<&str> {
        self.inner.path.as_deref()
    }
    pub fn route(&self) -> Option<&str> {
        self.inner.route.as_deref()
    }
    pub fn status_code(&self) -> Option<u16> {
        self.inner.status_code
    }
    pub fn attrs(&self) -> &Map<String, Value> {
        &self.inner.attrs
    }
    pub fn http(&self) -> &Map<String, Value> {
        &self.inner.http
    }
    pub fn user(&self) -> &Map<String, Value> {
        &self.inner.user
    }
    pub fn tenant(&self) -> &Map<String, Value> {
        &self.inner.tenant
    }
    pub fn resource(&self) -> &Map<String, Value> {
        &self.inner.resource
    }
    pub fn error(&self) -> Option<&Map<String, Value>> {
        self.inner.error.as_ref()
    }
    pub fn checkpoints(&self) -> &[Map<String, Value>] {
        &self.inner.checkpoints
    }
    pub fn is_finished(&self) -> bool {
        self.inner.is_finished()
    }
    pub fn is_emitted(&self) -> bool {
        self.inner.is_emitted()
    }
    pub fn lifecycle_state(&self) -> String {
        self.inner.lifecycle_state()
    }
    pub fn attr(&self, key: &str) -> Option<&Value> {
        lookup_path(&self.inner.attrs, key)
    }
}

impl<'a> std::fmt::Debug for EventView<'a> {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("EventView")
            .field("event_id", &self.inner.event_id)
            .field("event", &self.inner.event)
            .field("service", &self.inner.service)
            .finish()
    }
}

fn lookup_path<'a>(map: &'a Map<String, Value>, key: &str) -> Option<&'a Value> {
    let parts: Vec<&str> = key.split('.').collect();
    let mut current: &Map<String, Value> = map;
    for (i, part) in parts.iter().enumerate() {
        match current.get(*part) {
            Some(value) if i == parts.len() - 1 => return Some(value),
            Some(Value::Object(child)) => current = child,
            _ => return None,
        }
    }
    None
}

#[derive(Clone, Debug)]
pub struct DefaultSchemaType(pub SchemaConfig);

impl Schema for DefaultSchemaType {
    fn encode(&self, event: &EventContext) -> Value {
        encode_schema(event, &self.0)
    }
}

pub fn encode_schema(event: &EventContext, schema: &SchemaConfig) -> Value {
    let value =
        normalize_default_value(serde_json::to_value(event).unwrap_or(Value::Object(Map::new())));
    match schema {
        SchemaConfig::Default | SchemaConfig::Nested => value,
        SchemaConfig::Flat => flatten_value(value),
        SchemaConfig::OTel => otel_value(value),
        SchemaConfig::ECS => ecs_value(value),
        SchemaConfig::Datadog => datadog_value(value),
        SchemaConfig::Custom(schema) => schema.encode(event),
    }
}

pub fn encode_schema_from_value(value: Value, schema: &SchemaConfig) -> Value {
    let value = normalize_default_value(value);
    match schema {
        SchemaConfig::Default | SchemaConfig::Nested => value,
        SchemaConfig::Flat => flatten_value(value),
        SchemaConfig::OTel => otel_value(value),
        SchemaConfig::ECS => ecs_value(value),
        SchemaConfig::Datadog => datadog_value(value),
        SchemaConfig::Custom(_) => value,
    }
}

fn normalize_default_value(value: Value) -> Value {
    let mut obj = match value {
        Value::Object(obj) => obj,
        other => return other,
    };

    let mut http = match obj.remove("http") {
        Some(Value::Object(map)) => map,
        _ => Map::new(),
    };
    copy_scalar_field(&obj, &mut http, "method");
    copy_scalar_field(&obj, &mut http, "path");
    copy_scalar_field(&obj, &mut http, "route");
    copy_scalar_field(&obj, &mut http, "status_code");
    if !http.is_empty() {
        obj.insert("http".to_string(), Value::Object(http));
    }

    Value::Object(obj)
}

fn copy_scalar_field(source: &Map<String, Value>, target: &mut Map<String, Value>, key: &str) {
    if target.contains_key(key) {
        return;
    }
    if let Some(value) = source.get(key) {
        target.insert(key.to_string(), value.clone());
    }
}

fn flatten_value(value: Value) -> Value {
    let mut out = Map::new();
    flatten_into("", &value, &mut out);
    Value::Object(out)
}

fn flatten_into(prefix: &str, value: &Value, out: &mut Map<String, Value>) {
    match value {
        Value::Object(map) => {
            for (key, child) in map {
                let next = if prefix.is_empty() {
                    key.to_string()
                } else {
                    format!("{prefix}_{key}")
                };
                flatten_into(&next, child, out);
            }
        }
        _ => {
            out.insert(prefix.to_string(), value.clone());
        }
    }
}

fn otel_value(value: Value) -> Value {
    let mut attrs = value.as_object().cloned().unwrap_or_default();
    let timestamp = attrs.remove("timestamp").unwrap_or(Value::Null);
    let level = attrs
        .remove("level")
        .unwrap_or(Value::String("info".to_string()));
    let body = attrs
        .get("message")
        .cloned()
        .or_else(|| attrs.get("event").cloned())
        .unwrap_or(Value::Null);
    serde_json::json!({
        "time_unix_nano": timestamp,
        "severity_text": level,
        "body": body,
        "attributes": attrs,
    })
}

fn ecs_value(value: Value) -> Value {
    let obj = value.as_object().cloned().unwrap_or_default();
    serde_json::json!({
        "@timestamp": obj.get("timestamp").cloned().unwrap_or(Value::Null),
        "event": {
            "id": obj.get("event_id").cloned().unwrap_or(Value::Null),
            "action": obj.get("event").cloned().unwrap_or(Value::Null),
            "kind": obj.get("kind").cloned().unwrap_or(Value::Null),
            "outcome": obj.get("outcome").cloned().unwrap_or(Value::Null),
            "duration": obj.get("duration_ms").cloned().unwrap_or(Value::Null),
        },
        "log": {"level": obj.get("level").cloned().unwrap_or(Value::Null)},
        "service": {"name": obj.get("service").cloned().unwrap_or(Value::Null)},
        "labels": obj.get("attrs").cloned().unwrap_or(Value::Object(Map::new())),
    })
}

fn datadog_value(value: Value) -> Value {
    let mut obj = value.as_object().cloned().unwrap_or_default();
    obj.insert("ddsource".to_string(), Value::String("loxa".to_string()));
    obj.insert(
        "status".to_string(),
        obj.get("level")
            .cloned()
            .unwrap_or(Value::String("info".to_string())),
    );
    Value::Object(obj)
}
