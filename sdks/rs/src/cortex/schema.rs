use serde::{Deserialize, Serialize};

/// Result of incident reconstruction.
#[derive(Clone, Debug, Default, Serialize, Deserialize)]
#[serde(default)]
pub struct IncidentContext {
    pub incident_id: String,
    pub timestamp: String,
    pub causal_chain: Vec<serde_json::Value>,
    pub related_services: Vec<String>,
    pub symptoms: Vec<serde_json::Value>,
    pub suggested_actions: Vec<serde_json::Value>,
    pub confidence: f64,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub similar_incidents: Option<Vec<serde_json::Value>>,
}

/// Service or incident dependency graph.
#[derive(Clone, Debug, Default, Serialize, Deserialize)]
#[serde(default)]
pub struct GraphView {
    pub nodes: Vec<serde_json::Value>,
    pub edges: Vec<serde_json::Value>,
}

/// A remediation action taken for an incident.
#[derive(Clone, Debug, Default, Serialize, Deserialize)]
#[serde(default)]
pub struct Remediation {
    pub remediation_id: String,
    pub incident_id: String,
    pub action: String,
    pub timestamp: String,
    pub operator: String,
    pub attributes: serde_json::Value,
}

/// Feedback on whether a remediation worked.
#[derive(Clone, Debug, Default, Serialize, Deserialize)]
#[serde(default)]
pub struct RemediationFeedback {
    pub feedback_id: String,
    pub remediation_id: String,
    pub incident_id: String,
    pub outcome: String,
    pub time_to_resolve_seconds: i64,
    pub timestamp: String,
    pub notes: String,
}
