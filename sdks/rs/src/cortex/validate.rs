use super::schema::{GraphView, IncidentContext, Remediation, RemediationFeedback};

/// Validation error for cortex API requests.
#[derive(Clone, Debug)]
pub struct CortexValidationError {
    pub field: String,
    pub message: String,
}

impl std::fmt::Display for CortexValidationError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}: {}", self.field, self.message)
    }
}

impl std::error::Error for CortexValidationError {}

/// Validate an incident context before sending to cortex API.
/// Checks required fields are present and well-formed.
pub fn validate_incident_context(ctx: &IncidentContext) -> Result<(), CortexValidationError> {
    if ctx.incident_id.trim().is_empty() {
        return Err(CortexValidationError {
            field: "incident_id".to_string(),
            message: "must be non-empty".to_string(),
        });
    }
    if ctx.timestamp.trim().is_empty() {
        return Err(CortexValidationError {
            field: "timestamp".to_string(),
            message: "must be non-empty".to_string(),
        });
    }
    if ctx.confidence < 0.0 || ctx.confidence > 1.0 {
        return Err(CortexValidationError {
            field: "confidence".to_string(),
            message: "must be between 0.0 and 1.0".to_string(),
        });
    }
    Ok(())
}

/// Validate a graph view response from cortex API.
pub fn validate_graph_view(graph: &GraphView) -> Result<(), CortexValidationError> {
    for (i, node) in graph.nodes.iter().enumerate() {
        if node.get("id").is_none() {
            return Err(CortexValidationError {
                field: format!("nodes[{i}].id"),
                message: "each node must have an id".to_string(),
            });
        }
    }
    for (i, edge) in graph.edges.iter().enumerate() {
        if edge.get("source").is_none() || edge.get("target").is_none() {
            return Err(CortexValidationError {
                field: format!("edges[{i}]"),
                message: "each edge must have source and target".to_string(),
            });
        }
    }
    Ok(())
}

/// Validate remediation before recording.
pub fn validate_remediation(remediation: &Remediation) -> Result<(), CortexValidationError> {
    if remediation.incident_id.trim().is_empty() {
        return Err(CortexValidationError {
            field: "incident_id".to_string(),
            message: "must be non-empty".to_string(),
        });
    }
    if remediation.action.trim().is_empty() {
        return Err(CortexValidationError {
            field: "action".to_string(),
            message: "must be non-empty".to_string(),
        });
    }
    Ok(())
}

/// Validate remediation feedback before recording.
pub fn validate_remediation_feedback(
    feedback: &RemediationFeedback,
) -> Result<(), CortexValidationError> {
    if feedback.remediation_id.trim().is_empty() {
        return Err(CortexValidationError {
            field: "remediation_id".to_string(),
            message: "must be non-empty".to_string(),
        });
    }
    if feedback.incident_id.trim().is_empty() {
        return Err(CortexValidationError {
            field: "incident_id".to_string(),
            message: "must be non-empty".to_string(),
        });
    }
    if feedback.outcome.trim().is_empty() {
        return Err(CortexValidationError {
            field: "outcome".to_string(),
            message: "must be non-empty".to_string(),
        });
    }
    Ok(())
}
