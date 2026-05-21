package cortex

import (
	"strings"
	"time"
)

// NormalizeIncidentContext trims strings, clamps confidence, and fills empty timestamps.
func NormalizeIncidentContext(ctx *IncidentContext) {
	if ctx == nil {
		return
	}
	ctx.IncidentID = strings.TrimSpace(ctx.IncidentID)
	ctx.Timestamp = normalizeTimestamp(ctx.Timestamp)
	if ctx.Confidence < 0 {
		ctx.Confidence = 0
	}
	if ctx.Confidence > 1 {
		ctx.Confidence = 1
	}
	for i := range ctx.RelatedServices {
		ctx.RelatedServices[i] = strings.TrimSpace(ctx.RelatedServices[i])
	}
}

// NormalizeRemediation trims strings and normalizes the timestamp.
func NormalizeRemediation(r *Remediation) {
	if r == nil {
		return
	}
	r.RemediationID = strings.TrimSpace(r.RemediationID)
	r.IncidentID = strings.TrimSpace(r.IncidentID)
	r.Action = strings.TrimSpace(r.Action)
	r.Operator = strings.TrimSpace(r.Operator)
	r.Timestamp = normalizeTimestamp(r.Timestamp)
}

// NormalizeRemediationFeedback trims strings and normalizes the timestamp.
func NormalizeRemediationFeedback(rf *RemediationFeedback) {
	if rf == nil {
		return
	}
	rf.FeedbackID = strings.TrimSpace(rf.FeedbackID)
	rf.RemediationID = strings.TrimSpace(rf.RemediationID)
	rf.IncidentID = strings.TrimSpace(rf.IncidentID)
	rf.Outcome = strings.TrimSpace(rf.Outcome)
	rf.Notes = strings.TrimSpace(rf.Notes)
	rf.Timestamp = normalizeTimestamp(rf.Timestamp)
}

// NormalizeGraphNodes trims node IDs in a GraphView.
func NormalizeGraphNodes(gv *GraphView) {
	if gv == nil {
		return
	}
	for _, node := range gv.Nodes {
		if id, ok := node["id"].(string); ok {
			node["id"] = strings.TrimSpace(id)
		}
	}
}

func normalizeTimestamp(ts string) string {
	ts = strings.TrimSpace(ts)
	if ts == "" {
		return time.Now().UTC().Format(time.RFC3339)
	}
	return ts
}
