use serde_json::Value;
use std::collections::BTreeMap;
use std::io;
use std::time::Duration;

use crate::generated::spec_contract::{
    build_ingest_envelope, parse_collector_response_value,
    CollectorResponse as SpecCollectorResponse,
};

#[derive(Clone, Debug)]
pub struct HTTPRequest {
    pub method: String,
    pub url: String,
    pub headers: BTreeMap<String, String>,
    pub body: Option<Vec<u8>>,
}

impl HTTPRequest {
    pub fn new(method: impl Into<String>, url: impl Into<String>) -> Self {
        Self {
            method: method.into(),
            url: url.into(),
            headers: BTreeMap::new(),
            body: None,
        }
    }

    pub fn with_header(mut self, key: impl Into<String>, value: impl Into<String>) -> Self {
        self.headers.insert(key.into(), value.into());
        self
    }

    pub fn with_body(mut self, body: impl Into<Vec<u8>>) -> Self {
        self.body = Some(body.into());
        self
    }
}

#[derive(Clone, Debug)]
pub struct HTTPResponse {
    pub status_code: u16,
    pub headers: BTreeMap<String, String>,
    pub body: String,
}

#[derive(Clone, Debug)]
pub struct HTTPClient {
    agent: ureq::Agent,
}

impl Default for HTTPClient {
    fn default() -> Self {
        Self::new()
    }
}

impl HTTPClient {
    pub fn new() -> Self {
        Self {
            agent: ureq::AgentBuilder::new()
                .timeout(Duration::from_millis(2_000))
                .build(),
        }
    }

    pub fn with_timeout_ms(timeout_ms: u64) -> Self {
        Self {
            agent: ureq::AgentBuilder::new()
                .timeout(Duration::from_millis(timeout_ms))
                .build(),
        }
    }

    pub fn send(&self, request: &HTTPRequest) -> io::Result<HTTPResponse> {
        self.send_raw(request)
    }

    pub fn send_with_context(
        &self,
        ctx: &mut crate::EventContext,
        request: &HTTPRequest,
    ) -> io::Result<HTTPResponse> {
        let mut request = request.clone();
        inject_http_headers(&mut request, ctx);

        // Add "http.client.started" checkpoint
        let mut started = serde_json::Map::new();
        started.insert(
            "name".to_string(),
            serde_json::Value::String("http.client.started".to_string()),
        );
        started.insert(
            "http.client.method".to_string(),
            serde_json::Value::String(request.method.clone()),
        );
        ctx.checkpoints.push(started);

        let response = self.send_raw(&request)?;

        // Add "http.client.finished" checkpoint
        let mut finished = serde_json::Map::new();
        finished.insert(
            "name".to_string(),
            serde_json::Value::String("http.client.finished".to_string()),
        );
        finished.insert(
            "http.client.status_code".to_string(),
            serde_json::Value::Number(response.status_code.into()),
        );
        ctx.checkpoints.push(finished);

        Ok(response)
    }

    fn send_raw(&self, request: &HTTPRequest) -> io::Result<HTTPResponse> {
        let mut req = match request.method.to_uppercase().as_str() {
            "GET" => self.agent.get(&request.url),
            "POST" => self.agent.post(&request.url),
            "PUT" => self.agent.put(&request.url),
            "DELETE" => self.agent.delete(&request.url),
            other => {
                return Err(io::Error::new(
                    io::ErrorKind::InvalidInput,
                    format!("unsupported method: {other}"),
                ))
            }
        };
        for (key, value) in &request.headers {
            req = req.set(key, value);
        }
        let response = if let Some(body) = &request.body {
            req.send_bytes(body)
        } else {
            req.call()
        };
        let response = response.map_err(|err| io::Error::new(io::ErrorKind::Other, err))?;
        let status_code = response.status();
        let mut headers = BTreeMap::new();
        for name in response.headers_names() {
            if let Some(value) = response.header(&name) {
                headers.insert(name, value.to_string());
            }
        }
        let body = response
            .into_string()
            .map_err(|err| io::Error::new(io::ErrorKind::Other, err))?;
        Ok(HTTPResponse {
            status_code,
            headers,
            body,
        })
    }
}

/// Collector-specific HTTP client with envelope support.
#[derive(Clone, Debug)]
pub struct CollectorHttpClient {
    pub endpoint: String,
    pub api_key: Option<String>,
    pub auth_header: String,
    pub timeout_ms: u64,
    pub sdk_name: String,
    pub sdk_version: String,
    pub service: Option<String>,
}

impl CollectorHttpClient {
    pub fn new(endpoint: impl Into<String>) -> Self {
        Self {
            endpoint: endpoint.into(),
            api_key: None,
            auth_header: "Authorization".to_string(),
            timeout_ms: 2_000,
            sdk_name: "loxa-rs".to_string(),
            sdk_version: "1.0.0".to_string(),
            service: None,
        }
    }

    pub fn with_api_key(mut self, api_key: impl Into<String>) -> Self {
        self.api_key = Some(api_key.into());
        self
    }

    pub fn with_timeout_ms(mut self, timeout_ms: u64) -> Self {
        self.timeout_ms = timeout_ms;
        self
    }

    pub fn with_service(mut self, service: impl Into<String>) -> Self {
        self.service = Some(service.into());
        self
    }

    /// Build an ingest envelope from encoded events.
    pub fn envelope(&self, encoded_events: &[String]) -> Value {
        let events: Vec<Value> = encoded_events
            .iter()
            .filter_map(|event| serde_json::from_str(event).ok())
            .collect();
        let service = self
            .service
            .clone()
            .or_else(|| infer_service(&events))
            .unwrap_or_else(|| "unknown".to_string());
        build_ingest_envelope(&events, &self.sdk_name, &self.sdk_version, &service)
    }

    /// Validate an ingest envelope.
    pub fn validate_envelope(&self, envelope: &Value) -> Result<(), String> {
        validate_ingest_envelope(envelope)
    }

    /// Parse a collector response.
    pub fn parse_response(&self, body: &str) -> Result<SpecCollectorResponse, serde_json::Error> {
        let value: Value = serde_json::from_str(body)?;
        parse_collector_response_value(&value)
    }

    /// Get the tail endpoint URL.
    pub fn tail_endpoint(&self) -> String {
        format!("{}/tail", self.endpoint.trim_end_matches('/'))
    }
}

/// Collector response wrapper.
#[derive(Clone, Debug)]
pub struct CollectorResponse {
    pub status_code: u16,
    pub body: Value,
}

/// Inject trace context into HTTP headers for propagation.
pub fn inject_http_headers(request: &mut HTTPRequest, event: &crate::EventContext) {
    if let (Some(trace_id), Some(span_id)) = (&event.trace_id, &event.span_id) {
        if trace_id.len() == 32 && span_id.len() == 16 {
            let traceparent = format!(
                "00-{}-{}-01",
                trace_id.to_lowercase(),
                span_id.to_lowercase()
            );
            request
                .headers
                .insert("traceparent".to_string(), traceparent);
        }
    }
    if let Some(trace_id) = &event.trace_id {
        request
            .headers
            .insert("x-trace-id".to_string(), trace_id.clone());
    }
    if let Some(span_id) = &event.span_id {
        request
            .headers
            .insert("x-span-id".to_string(), span_id.clone());
    }
    request
        .headers
        .insert("x-request-id".to_string(), event.request_id.clone());
}

/// Extract trace context from HTTP headers.
pub fn extract_http_headers(
    headers: &BTreeMap<String, String>,
) -> (Option<String>, Option<String>, Option<String>) {
    let trace_id = headers.get("x-trace-id").cloned();
    let span_id = headers.get("x-span-id").cloned();
    let request_id = headers.get("x-request-id").cloned();
    (trace_id, span_id, request_id)
}

fn infer_service(events: &[Value]) -> Option<String> {
    for event in events {
        if let Some(service) = event.get("service").and_then(Value::as_str) {
            if !service.is_empty() {
                return Some(service.to_string());
            }
        }
    }
    None
}

/// Validate an ingest envelope against the spec.
pub fn validate_ingest_envelope(envelope: &Value) -> Result<(), String> {
    let object = envelope
        .as_object()
        .ok_or_else(|| "collector envelope must be a JSON object".to_string())?;

    let api_version = object
        .get("api_version")
        .and_then(Value::as_str)
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .ok_or_else(|| "collector envelope must include api_version".to_string())?;
    if api_version != crate::generated::spec_contract::LOXA_INGEST_API_VERSION {
        return Err(format!(
            "collector envelope api_version must be {}",
            crate::generated::spec_contract::LOXA_INGEST_API_VERSION
        ));
    }

    let source = object
        .get("source")
        .and_then(Value::as_object)
        .ok_or_else(|| "collector envelope must include a source object".to_string())?;
    for key in ["sdk", "version", "service"] {
        match source.get(key).and_then(Value::as_str).map(str::trim) {
            Some(value) if !value.is_empty() => {}
            _ => {
                return Err(format!(
                    "collector envelope source.{key} must be a non-empty string"
                ))
            }
        }
    }

    let events = object
        .get("events")
        .and_then(Value::as_array)
        .ok_or_else(|| "collector envelope must include an events array".to_string())?;
    if events.is_empty() {
        return Err("collector envelope must include at least one event".to_string());
    }
    for (idx, event) in events.iter().enumerate() {
        if event.as_object().is_none() {
            return Err(format!(
                "collector envelope events[{idx}] must be JSON objects"
            ));
        }
    }

    Ok(())
}
