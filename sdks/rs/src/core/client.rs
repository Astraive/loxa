use base64::Engine as _;
use serde::{Deserialize, Serialize};
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
        let response = response.map_err(io::Error::other)?;
        let status_code = response.status();
        let mut headers = BTreeMap::new();
        for name in response.headers_names() {
            if let Some(value) = response.header(&name) {
                headers.insert(name, value.to_string());
            }
        }
        let body = response.into_string().map_err(io::Error::other)?;
        Ok(HTTPResponse {
            status_code,
            headers,
            body,
        })
    }
}

/// Collector-specific HTTP client with envelope support.
#[derive(Clone)]
pub struct CollectorHttpClient {
    pub endpoint: String,
    pub collector_name: Option<String>,
    pub environment: Option<String>,
    pub api_key: Option<String>,
    pub basic_username: Option<String>,
    pub basic_password: Option<String>,
    pub insecure: bool,
    pub auth_header: String,
    pub timeout_ms: u64,
    pub sdk_name: String,
    pub sdk_version: String,
    pub service: Option<String>,
}
impl std::fmt::Debug for CollectorHttpClient {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("CollectorHttpClient")
            .field("endpoint", &self.endpoint)
            .field("api_key", &self.api_key.as_ref().map(|_| "<redacted>"))
            .field(
                "basic_credentials",
                &self.basic_username.as_ref().map(|_| "<redacted>"),
            )
            .field("insecure", &self.insecure)
            .field("auth_header", &self.auth_header)
            .field("timeout_ms", &self.timeout_ms)
            .field("sdk_name", &self.sdk_name)
            .field("sdk_version", &self.sdk_version)
            .field("service", &self.service)
            .finish()
    }
}

impl CollectorHttpClient {
    pub fn new(endpoint: impl Into<String>) -> Self {
        let endpoint = normalize_collector_endpoint(endpoint.into());
        Self {
            endpoint,
            collector_name: None,
            environment: None,
            api_key: None,
            basic_username: None,
            basic_password: None,
            insecure: false,
            auth_header: "Authorization".to_string(),
            timeout_ms: 2_000,
            sdk_name: "loza-rs".to_string(),
            sdk_version: crate::core::version::sdk_version().to_string(),
            service: None,
        }
    }

    pub fn with_api_key(mut self, api_key: impl Into<String>) -> Self {
        self.api_key = Some(api_key.into());
        self
    }

    pub fn with_collector(mut self, collector: impl Into<String>) -> Self {
        self.collector_name = Some(collector.into());
        self
    }

    pub fn with_environment(mut self, environment: impl Into<String>) -> Self {
        self.environment = Some(environment.into());
        self
    }

    pub fn with_basic_auth(
        mut self,
        username: impl Into<String>,
        password: impl Into<String>,
    ) -> Self {
        self.basic_username = Some(username.into());
        self.basic_password = Some(password.into());
        self
    }

    pub fn with_insecure(mut self, insecure: bool) -> Self {
        self.insecure = insecure;
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

    /// Send an authenticated HTTP request to the collector.
    fn request(
        &self,
        method: &str,
        path: &str,
        body: Option<Value>,
    ) -> Result<CollectorResponse, String> {
        let url = format!("{}{}", self.endpoint.trim_end_matches('/'), path);
        let client = HTTPClient::with_timeout_ms(self.timeout_ms);
        let mut request = HTTPRequest::new(method, &url);

        if let Some(api_key) = &self.api_key {
            request = request.with_header(&self.auth_header, format!("Bearer {}", api_key));
        } else {
            match (&self.basic_username, &self.basic_password) {
                (Some(username), Some(password)) => {
                    validate_basic_auth_endpoint(
                        &self.endpoint,
                        username,
                        password,
                        self.insecure,
                    )?;
                    let token = base64::engine::general_purpose::STANDARD
                        .encode(format!("{username}:{password}"));
                    request = request.with_header(&self.auth_header, format!("Basic {token}"));
                }
                (None, None) => {}
                _ => {
                    return Err("basic username and password must be configured together".into());
                }
            }
        }

        request = request
            .with_header(
                "User-Agent",
                format!("{}/{}", self.sdk_name, self.sdk_version),
            )
            .with_header("Content-Type", "application/json");
        if let Some(environment) = &self.environment {
            request = request.with_header("X-Loza-Env", environment);
        }
        if let Some(service) = &self.service {
            request = request.with_header("X-Loza-Service", service);
        }

        if let Some(body) = body {
            let bytes = serde_json::to_vec(&body).map_err(|e| e.to_string())?;
            request = request.with_body(bytes);
        }

        let response = client.send(&request).map_err(|e| e.to_string())?;
        let body: Value = serde_json::from_str(&response.body).map_err(|e| e.to_string())?;
        Ok(CollectorResponse {
            status_code: response.status_code,
            body,
        })
    }

    /// Validate events locally against the ingest envelope contract.
    /// The collector does not expose a dedicated `/validate` endpoint.
    pub fn validate(&self, events: &[String]) -> Result<CollectorResponse, String> {
        let envelope = self.envelope(events);
        self.validate_envelope(&envelope)?;
        Ok(CollectorResponse {
            status_code: 200,
            body: serde_json::json!({
                "status": "accepted",
                "valid": true
            }),
        })
    }

    /// Ingest events into the collector.
    pub fn ingest(&self, events: &[String]) -> Result<CollectorResponse, String> {
        let envelope = self.envelope(events);
        self.request("POST", "/events", Some(envelope))
    }

    /// Query events from the collector.
    pub fn query(&self, query: &str) -> Result<CollectorResponse, String> {
        self.request("POST", "/query", Some(serde_json::json!({"query": query})))
    }

    /// Tail recent events from the collector.
    pub fn tail(&self, count: u32) -> Result<CollectorResponse, String> {
        self.request("GET", &format!("/tail?limit={}", count), None)
    }

    /// Delete events from the collector by event ID.
    pub fn delete(&self, event_id: &str) -> Result<CollectorResponse, String> {
        self.request("DELETE", &format!("/events/{}", event_id), None)
    }

    /// Replay events from the dead-letter queue.
    pub fn replay(&self, event_ids: &[String]) -> Result<CollectorResponse, String> {
        self.request(
            "POST",
            "/replay",
            Some(serde_json::json!({"event_ids": event_ids})),
        )
    }

    /// List dead-letter queue entries.
    pub fn dlq_list(&self, limit: u32) -> Result<CollectorResponse, String> {
        self.request("GET", &format!("/dlq?limit={}", limit), None)
    }

    /// Read a dead-letter queue entry.
    pub fn dlq_read(&self, entry_id: &str) -> Result<CollectorResponse, String> {
        self.request("GET", &format!("/dlq/{}", entry_id), None)
    }

    /// Replay specific dead-letter queue entries.
    pub fn dlq_replay(&self, entry_ids: &[String]) -> Result<CollectorResponse, String> {
        self.request(
            "POST",
            "/dlq/replay",
            Some(serde_json::json!({"entry_ids": entry_ids})),
        )
    }

    /// Create an API key.
    pub fn keys_create(&self, name: &str) -> Result<CollectorResponse, String> {
        self.request("POST", "/keys", Some(serde_json::json!({"name": name})))
    }

    /// Revoke an API key.
    pub fn keys_revoke(&self, key_id: &str) -> Result<CollectorResponse, String> {
        self.request("DELETE", &format!("/keys/{}", key_id), None)
    }

    /// List configured sinks on the collector.
    pub fn sinks_list(&self) -> Result<CollectorResponse, String> {
        self.request("GET", "/sinks", None)
    }

    /// Test a configured sink on the collector.
    pub fn sinks_test(&self, name: &str) -> Result<CollectorResponse, String> {
        self.request("POST", &format!("/sinks/{}/test", name), None)
    }

    /// Rotate an API key.
    pub fn keys_rotate(&self, key_id: &str) -> Result<CollectorResponse, String> {
        self.request("POST", &format!("/keys/{}/rotate", key_id), None)
    }

    /// Validate an event governance policy.
    pub fn policy_validate(&self, policy: &Value) -> Result<CollectorResponse, String> {
        self.request("POST", "/policy/validate", Some(policy.clone()))
    }

    /// Check an event against the active schema.
    pub fn schema_check(&self, event: &Value) -> Result<CollectorResponse, String> {
        self.request("POST", "/schema/check", Some(event.clone()))
    }

    /// Publish schema metadata.
    pub fn schema_publish(&self, schema: &Value) -> Result<CollectorResponse, String> {
        self.request("POST", "/schema/publish", Some(schema.clone()))
    }

    /// Apply retention policy immediately.
    pub fn retention_apply(&self, policy: &Value) -> Result<CollectorResponse, String> {
        self.request("POST", "/retention/apply", Some(policy.clone()))
    }

    /// Check collector health.
    pub fn health(&self) -> Result<CollectorResponse, String> {
        self.request("GET", "/health", None)
    }
    pub fn query_lql(
        &self,
        source: &str,
        parameters: std::collections::HashMap<String, QueryValue>,
        limit: usize,
    ) -> Result<QueryResult, QueryLqlError> {
        if source.trim().is_empty() {
            return Err(QueryLqlError::new(
                "invalid_configuration",
                "LQL query source is required",
                None,
            ));
        }
        let route = self
            .collector_name
            .as_ref()
            .map(|collector| format!("/collectors/{collector}/lql/query"))
            .unwrap_or_else(|| "/lql/query".to_string());
        let response = self
            .request(
                "POST",
                &route,
                Some(serde_json::json!({
                    "query": source,
                    "parameters": parameters,
                    "limit": limit.clamp(1, 1000)
                })),
            )
            .map_err(|error| {
                QueryLqlError::new("transport", "LQL query transport failed", Some(error))
            })?;
        if response.status_code >= 300 {
            let category = match response.status_code {
                400 => "diagnostics",
                401 => "authentication",
                403 => "scope",
                503 => "compiler_unavailable",
                _ => "execution",
            };
            return Err(QueryLqlError::new(
                category,
                "LQL query failed",
                Some(response.status_code.to_string()),
            ));
        }
        let columns = response
            .body
            .get("columns")
            .and_then(Value::as_array)
            .ok_or_else(|| {
                QueryLqlError::new(
                    "malformed_response",
                    "LQL response has invalid columns",
                    None,
                )
            })?
            .iter()
            .map(|column| {
                if let Some(name) = column.as_str() {
                    Ok(QueryColumn {
                        name: name.into(),
                        value_type: String::new(),
                    })
                } else {
                    serde_json::from_value(column.clone()).map_err(|_| {
                        QueryLqlError::new(
                            "malformed_response",
                            "LQL response has invalid columns",
                            None,
                        )
                    })
                }
            })
            .collect::<Result<Vec<_>, _>>()?;
        let rows =
            serde_json::from_value(response.body.get("rows").cloned().unwrap_or(Value::Null))
                .map_err(|_| {
                    QueryLqlError::new("malformed_response", "LQL response has invalid rows", None)
                })?;
        let row_count = response
            .body
            .get("row_count")
            .and_then(Value::as_u64)
            .unwrap_or(0) as usize;
        let duration_ms = response
            .body
            .get("duration_ms")
            .and_then(Value::as_i64)
            .unwrap_or(0);
        Ok(QueryResult {
            columns,
            rows,
            duration_ms,
            row_count,
        })
    }
}
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct QueryValue {
    #[serde(rename = "type")]
    pub value_type: String,
    pub value: Value,
}

impl QueryValue {
    pub fn new(value_type: impl Into<String>, value: impl Serialize) -> Self {
        Self {
            value_type: value_type.into(),
            value: serde_json::to_value(value).unwrap_or(Value::Null),
        }
    }
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct QueryColumn {
    pub name: String,
    #[serde(default, rename = "type")]
    pub value_type: String,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct QueryResult {
    pub columns: Vec<QueryColumn>,
    pub rows: Vec<std::collections::HashMap<String, Value>>,
    #[serde(default)]
    pub duration_ms: i64,
    #[serde(default)]
    pub row_count: usize,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct QueryLqlError {
    pub category: String,
    pub message: String,
    pub status: Option<String>,
}

impl QueryLqlError {
    fn new(category: &str, message: &str, status: Option<String>) -> Self {
        Self {
            category: category.into(),
            message: message.into(),
            status,
        }
    }
}

impl std::fmt::Display for QueryLqlError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.message)
    }
}

impl std::error::Error for QueryLqlError {}

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
fn validate_basic_auth_endpoint(
    endpoint: &str,
    username: &str,
    _password: &str,
    insecure: bool,
) -> Result<(), String> {
    if username.is_empty() || username.contains(':') || username.chars().any(char::is_whitespace) {
        return Err(
            "basic auth username must be non-empty and contain no ':' or whitespace".into(),
        );
    }
    let Some((scheme, rest)) = endpoint.split_once("://") else {
        return Ok(());
    };
    let authority = rest.split(['/', '?', '#']).next().unwrap_or_default();
    if authority.contains('@') {
        return Err("collector credentials must not be embedded in the endpoint URL".into());
    }
    if !scheme.eq_ignore_ascii_case("http") || insecure {
        return Ok(());
    }
    let host = if let Some(ipv6) = authority.strip_prefix('[') {
        ipv6.split(']').next().unwrap_or(ipv6)
    } else {
        authority.split(':').next().unwrap_or(authority)
    };
    if matches!(host, "localhost" | "127.0.0.1" | "::1") {
        return Ok(());
    }
    Err("credentialed HTTP requires TLS (set insecure explicitly for local development)".into())
}

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

fn normalize_collector_endpoint(endpoint: String) -> String {
    let mut base = endpoint.trim().trim_end_matches('/').to_string();
    for suffix in ["/events", "/events/batch"] {
        if base.ends_with(suffix) {
            base.truncate(base.len() - suffix.len());
            break;
        }
    }
    base.trim_end_matches('/').to_string()
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
    if api_version != crate::generated::spec_contract::LOZA_INGEST_API_VERSION {
        return Err(format!(
            "collector envelope api_version must be {}",
            crate::generated::spec_contract::LOZA_INGEST_API_VERSION
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
