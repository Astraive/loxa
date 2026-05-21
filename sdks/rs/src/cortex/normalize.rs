use super::schema::{IncidentContext, Remediation, RemediationFeedback};
use serde_json::Value;

/// Normalize an incident context for sending to cortex API.
/// Ensures timestamps are RFC3339, trims whitespace, fills defaults.
pub fn normalize_incident_context(ctx: &mut IncidentContext) {
    ctx.incident_id = ctx.incident_id.trim().to_string();
    ctx.timestamp = normalize_timestamp(&ctx.timestamp);
    ctx.confidence = ctx.confidence.clamp(0.0, 1.0);

    for service in &mut ctx.related_services {
        *service = service.trim().to_string();
    }
    ctx.related_services.retain(|s| !s.is_empty());
}

/// Normalize a remediation for sending to cortex API.
pub fn normalize_remediation(remediation: &mut Remediation) {
    remediation.incident_id = remediation.incident_id.trim().to_string();
    remediation.action = remediation.action.trim().to_string();
    remediation.operator = remediation.operator.trim().to_string();
    remediation.timestamp = normalize_timestamp(&remediation.timestamp);
}

/// Normalize remediation feedback for sending to cortex API.
pub fn normalize_remediation_feedback(feedback: &mut RemediationFeedback) {
    feedback.remediation_id = feedback.remediation_id.trim().to_string();
    feedback.incident_id = feedback.incident_id.trim().to_string();
    feedback.outcome = feedback.outcome.trim().to_string();
    feedback.notes = feedback.notes.trim().to_string();
    feedback.timestamp = normalize_timestamp(&feedback.timestamp);
}

/// Normalize graph node IDs to lowercase trimmed strings.
pub fn normalize_graph_nodes(nodes: &mut [Value]) {
    for node in nodes {
        if let Some(id) = node.get("id").and_then(Value::as_str) {
            let normalized = id.trim().to_string();
            node["id"] = Value::String(normalized);
        }
    }
}

/// Normalize a timestamp string. If empty, uses current UTC time.
/// Passes through RFC3339 strings unchanged.
fn normalize_timestamp(ts: &str) -> String {
    let trimmed = ts.trim();
    if trimmed.is_empty() {
        // Use current UTC time
        time::OffsetDateTime::now_utc()
            .format(&time::format_description::well_known::Rfc3339)
            .unwrap_or_else(|_| "1970-01-01T00:00:00Z".to_string())
    } else {
        trimmed.to_string()
    }
}
