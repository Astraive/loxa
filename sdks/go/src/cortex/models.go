package cortex

// IncidentContext is the result of incident reconstruction from cortex.
type IncidentContext struct {
	IncidentID       string            `json:"incident_id"`
	Timestamp        string            `json:"timestamp"`
	CausalChain      []map[string]any  `json:"causal_chain"`
	RelatedServices  []string          `json:"related_services"`
	Symptoms         []map[string]any  `json:"symptoms"`
	SuggestedActions []map[string]any  `json:"suggested_actions"`
	Confidence       float64           `json:"confidence"`
	SimilarIncidents []map[string]any  `json:"similar_incidents,omitempty"`
	Explain          string            `json:"explain,omitempty"`
	ExplainReport    *ExplainReport    `json:"explain_report,omitempty"`
}

// ExplainReport is a structured explanation of incident reconstruction.
type ExplainReport struct {
	Narrative           string         `json:"narrative,omitempty"`
	ConfidenceBreakdown map[string]any `json:"confidence_breakdown,omitempty"`
	KeyFindings         []string       `json:"key_findings,omitempty"`
	DataGaps            []string       `json:"data_gaps,omitempty"`
	AlternativeHypotheses []string     `json:"alternative_hypotheses,omitempty"`
}

// GraphView represents a service or incident dependency graph.
type GraphView struct {
	Nodes []map[string]any `json:"nodes"`
	Edges []map[string]any `json:"edges"`
}

// Remediation records a remediation action taken for an incident.
type Remediation struct {
	RemediationID string         `json:"remediation_id"`
	IncidentID    string         `json:"incident_id"`
	SignatureID   string         `json:"signature_id,omitempty"`
	Action        string         `json:"action"`
	Timestamp     string         `json:"timestamp"`
	Operator      string         `json:"operator"`
	Attributes    map[string]any `json:"attributes"`
}

// RemediationFeedback records the outcome of a remediation action.
type RemediationFeedback struct {
	FeedbackID           string `json:"feedback_id"`
	RemediationID        string `json:"remediation_id"`
	IncidentID           string `json:"incident_id"`
	Outcome              string `json:"outcome"`
	TimeToResolveSeconds int64  `json:"time_to_resolve_seconds"`
	Timestamp            string `json:"timestamp"`
	Notes                string `json:"notes"`
}
