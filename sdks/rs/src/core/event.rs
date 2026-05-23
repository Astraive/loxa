pub use crate::event::{Attr, ContextCarrier, ContextSource, EventContext, Params};

use serde_json::{Map, Value};

pub fn group(name: impl Into<String>, attrs: impl IntoIterator<Item = Attr>) -> Attr {
    let mut object = Map::new();
    for attr in attrs {
        object.insert(attr.key, attr.value);
    }
    Attr::new(name.into(), Value::Object(object))
}

pub fn sensitive_string(key: impl Into<String>, value: impl Into<String>) -> Attr {
    Attr::new(key.into(), value.into()).sensitive()
}

pub fn hash_string(key: impl Into<String>, value: impl Into<String>) -> Attr {
    Attr::new(key.into(), value.into()).hash_value()
}

pub fn from_context(ctx: &EventContext) -> Option<&EventContext> {
    Some(ctx)
}

pub fn has_event(ctx: &EventContext) -> bool {
    !ctx.is_emitted()
}

pub fn event_id(ctx: &EventContext) -> &str {
    &ctx.event_id
}

pub fn request_id(ctx: &EventContext) -> &str {
    &ctx.request_id
}

pub fn id(event: &EventContext) -> &str {
    &event.event_id
}

pub fn is_finished(event: &EventContext) -> bool {
    event.is_finished()
}

pub fn is_emitted(event: &EventContext) -> bool {
    event.is_emitted()
}

pub fn checkpoint_names(event: &EventContext) -> Vec<String> {
    event
        .checkpoints
        .iter()
        .filter_map(|c| c.get("name").and_then(Value::as_str).map(str::to_string))
        .collect()
}

pub fn checkpoint_payload(name: &str, at_ms: u64) -> serde_json::Map<String, Value> {
    let mut payload = serde_json::Map::new();
    payload.insert("name".to_string(), Value::String(name.to_string()));
    payload.insert("at_ms".to_string(), Value::Number(at_ms.into()));
    payload
}

pub fn params_from_http(method: &str, path: &str, route: Option<&str>) -> Params {
    let mut params = Params::new(format!("{method} {path}")).with_kind("http");
    params.method = Some(method.to_string());
    params.path = Some(path.to_string());
    params.route = route.map(str::to_string);
    params
}
