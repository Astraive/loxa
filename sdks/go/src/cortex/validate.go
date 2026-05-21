package cortex

import (
	"fmt"
)

func edgeHasKey(edge map[string]any, key string) bool {
	_, ok := edge[key]
	return ok
}

// ValidateIncidentContext checks that an IncidentContext has required fields.
func ValidateIncidentContext(ctx *IncidentContext) error {
	if ctx == nil {
		return fmt.Errorf("cortex: incident context is nil")
	}
	if ctx.IncidentID == "" {
		return fmt.Errorf("cortex: incident_id is required")
	}
	if ctx.Timestamp == "" {
		return fmt.Errorf("cortex: timestamp is required")
	}
	if ctx.Confidence < 0 || ctx.Confidence > 1 {
		return fmt.Errorf("cortex: confidence must be in [0.0, 1.0], got %f", ctx.Confidence)
	}
	return nil
}

// ValidateGraphView checks that a GraphView has valid structure.
func ValidateGraphView(gv *GraphView) error {
	if gv == nil {
		return fmt.Errorf("cortex: graph view is nil")
	}
	for i, node := range gv.Nodes {
		if _, ok := node["id"]; !ok {
			return fmt.Errorf("cortex: node %d missing 'id'", i)
		}
	}
	for i, edge := range gv.Edges {
		hasSource := edgeHasKey(edge, "source") || edgeHasKey(edge, "from_node_id")
		hasTarget := edgeHasKey(edge, "target") || edgeHasKey(edge, "to_node_id")
		if !hasSource {
			return fmt.Errorf("cortex: edge %d missing 'source' or 'from_node_id'", i)
		}
		if !hasTarget {
			return fmt.Errorf("cortex: edge %d missing 'target' or 'to_node_id'", i)
		}
	}
	return nil
}

// ValidateRemediation checks that a Remediation has required fields.
func ValidateRemediation(r *Remediation) error {
	if r == nil {
		return fmt.Errorf("cortex: remediation is nil")
	}
	if r.IncidentID == "" {
		return fmt.Errorf("cortex: incident_id is required")
	}
	if r.Action == "" {
		return fmt.Errorf("cortex: action is required")
	}
	return nil
}

// ValidateRemediationFeedback checks that RemediationFeedback has required fields.
func ValidateRemediationFeedback(rf *RemediationFeedback) error {
	if rf == nil {
		return fmt.Errorf("cortex: remediation feedback is nil")
	}
	if rf.RemediationID == "" {
		return fmt.Errorf("cortex: remediation_id is required")
	}
	if rf.IncidentID == "" {
		return fmt.Errorf("cortex: incident_id is required")
	}
	if rf.Outcome == "" {
		return fmt.Errorf("cortex: outcome is required")
	}
	return nil
}
