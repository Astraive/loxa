use std::io;
use std::time::Duration;

use super::schema::{GraphView, IncidentContext, Remediation, RemediationFeedback};

/// Client for the Cortex incident intelligence API.
///
/// Provides methods for incident reconstruction, graph queries,
/// similar incident lookup, and remediation recording.
pub struct CortexClient {
    endpoint: String,
    api_key: Option<String>,
    auth_header: String,
    timeout: Duration,
}

impl CortexClient {
    pub fn new(endpoint: impl Into<String>) -> Self {
        Self {
            endpoint: endpoint.into().trim_end_matches('/').to_string(),
            api_key: None,
            auth_header: "x-loza-api-key".to_string(),
            timeout: Duration::from_secs(10),
        }
    }

    pub fn with_api_key(mut self, api_key: impl Into<String>) -> Self {
        self.api_key = Some(api_key.into());
        self
    }

    pub fn with_auth_header(mut self, header: impl Into<String>) -> Self {
        self.auth_header = header.into();
        self
    }

    pub fn with_timeout(mut self, timeout: Duration) -> Self {
        self.timeout = timeout;
        self
    }

    fn get(&self, path: &str) -> io::Result<serde_json::Value> {
        let url = format!("{}{}", self.endpoint, path);
        let mut request = ureq::get(&url).timeout(self.timeout);
        if let Some(key) = &self.api_key {
            request = request.set(&self.auth_header, key);
        }
        let response = request.call().map_err(io_err)?;
        let body: serde_json::Value = response.into_json().map_err(io_err)?;
        Ok(body)
    }

    fn post(&self, path: &str, body: &serde_json::Value) -> io::Result<serde_json::Value> {
        let url = format!("{}{}", self.endpoint, path);
        let mut request = ureq::post(&url)
            .set("Content-Type", "application/json")
            .timeout(self.timeout);
        if let Some(key) = &self.api_key {
            request = request.set(&self.auth_header, key);
        }
        let response = request.send_json(body).map_err(io_err)?;
        let resp_body: serde_json::Value = response.into_json().map_err(io_err)?;
        Ok(resp_body)
    }

    /// Check if cortex is healthy.
    pub fn health(&self) -> bool {
        self.get("/healthz")
            .ok()
            .and_then(|v| v.get("status").and_then(|s| s.as_str().map(|s| s == "ok")))
            .unwrap_or(false)
    }

    /// Check if cortex is ready to accept requests.
    pub fn ready(&self) -> bool {
        self.get("/readyz")
            .ok()
            .map(|v| {
                v.get("status").and_then(|s| s.as_str()) == Some("ok")
                    || v.get("ready").and_then(|r| r.as_bool()) == Some(true)
            })
            .unwrap_or(false)
    }

    /// Fetch Prometheus metrics from cortex.
    pub fn metrics(&self) -> io::Result<String> {
        let url = format!("{}/metrics", self.endpoint);
        let response = ureq::get(&url)
            .timeout(self.timeout)
            .call()
            .map_err(io_err)?;
        let body = response.into_string().map_err(io_err)?;
        Ok(body)
    }

    /// Reconstruct an incident timeline with root cause analysis.
    pub fn reconstruct(&self, incident_id: &str, mode: &str) -> io::Result<IncidentContext> {
        let body = serde_json::json!({
            "incident_id": incident_id,
            "mode": mode,
        });
        let resp = self.post("/reconstruct", &body)?;
        let ctx: IncidentContext = serde_json::from_value(resp).map_err(io_err)?;
        Ok(ctx)
    }

    /// Reconstruct an incident using the URL-param variant.
    pub fn reconstruct_incident(
        &self,
        incident_id: &str,
        mode: &str,
    ) -> io::Result<IncidentContext> {
        let path = format!("/incidents/{}/reconstruct", incident_id);
        let body = serde_json::json!({"mode": mode});
        let resp = self.post(&path, &body)?;
        let ctx: IncidentContext = serde_json::from_value(resp).map_err(io_err)?;
        Ok(ctx)
    }

    /// Fetch the dependency graph for a service.
    pub fn service_graph(&self, service: &str, depth: u32) -> io::Result<GraphView> {
        let path = format!("/graph/service/{}?depth={}", service, depth);
        let resp = self.get(&path)?;
        let graph: GraphView = serde_json::from_value(resp).map_err(io_err)?;
        Ok(graph)
    }

    /// Fetch the graph for a specific incident.
    pub fn incident_graph(&self, incident_id: &str, depth: u32) -> io::Result<GraphView> {
        let path = format!("/graph/incident/{}?depth={}", incident_id, depth);
        let resp = self.get(&path)?;
        let graph: GraphView = serde_json::from_value(resp).map_err(io_err)?;
        Ok(graph)
    }

    /// Record a remediation action taken for an incident.
    pub fn record_remediation(&self, remediation: &Remediation) -> io::Result<()> {
        let body = serde_json::json!({
            "incident_id": remediation.incident_id,
            "action": remediation.action,
            "operator": remediation.operator,
            "attributes": remediation.attributes,
        });
        self.post("/feedback/remediation", &body)?;
        Ok(())
    }

    /// Record feedback on whether a remediation was successful.
    pub fn record_feedback(&self, feedback: &RemediationFeedback) -> io::Result<()> {
        let body = serde_json::json!({
            "remediation_id": feedback.remediation_id,
            "incident_id": feedback.incident_id,
            "outcome": feedback.outcome,
            "time_to_resolve_seconds": feedback.time_to_resolve_seconds,
            "notes": feedback.notes,
        });
        self.post("/feedback/incident", &body)?;
        Ok(())
    }

    /// Find incidents similar to the given one.
    pub fn similar_incidents(&self, incident_id: &str) -> io::Result<Vec<serde_json::Value>> {
        let body = serde_json::json!({
            "incident_id": incident_id,
            "mode": "fast",
        });
        let resp = self.post("/reconstruct", &body)?;
        let similar = resp
            .get("similar_incidents")
            .and_then(|v| v.as_array())
            .cloned()
            .unwrap_or_default();
        Ok(similar)
    }

    /// Ingest a batch of events directly into cortex.
    pub fn ingest_batch(&self, events: &[serde_json::Value]) -> io::Result<()> {
        let body = serde_json::json!({"events": events});
        self.post("/events/batch", &body)?;
        Ok(())
    }
}

fn io_err<E: std::fmt::Display>(err: E) -> io::Error {
    io::Error::other(format!("cortex client error: {err}"))
}
