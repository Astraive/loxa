# CORTEX Feedback & Learning

## Feedback Types

### Remediation Feedback
```
POST /v1/feedback/remediation

{
  incident_id: string,
  remediation_id: string,
  success: boolean,
  outcome: "success" | "failed" | "partial" | "unknown",
  time_to_resolve_seconds: number,
  notes?: string
}
```

Records whether a specific remediation action fixed the incident.

### Incident Feedback
```
POST /v1/feedback/incident

{
  incident_id: string,
  actual_root_cause: string,
  severity_correction?: string,
  notes?: string
}
```

Corrects Cortex's incident analysis based on human review.

## Learning Loop

1. **Record Feedback** → Store in feedback table
2. **Extract Patterns** → Analyze remediation signatures
3. **Update Confidence** → Adjust edge weights based on correctness
4. **Train ML Models** → Improve future pattern matching
5. **Distribute Updates** → Deploy new ML signatures

## Signature Learning

Successful remediations contribute to learned signatures:

```
If: (metric spike after deployment) + (revert deployment)
    AND success=true
Then: Add/strengthen edge type "caused_probably" for future similar patterns
```

## Feedback Storage

See: `schemas/json/cortex-feedback.schema.json`

```
{
  feedback_id: string,
  incident_id: string,
  remediation_id: string,
  success: boolean,
  outcome: string,
  time_to_resolve_seconds: number,
  notes: string,
  timestamp: timestamp
}
```

## Feedback Impact on RCA

- Positive feedback (success=true) → increase edge weight for same pattern
- Negative feedback (success=false) → decrease edge weight
- Corrections (actual_root_cause) → adjust node confidence scores
- Patterns → train new ML model layer

## Privacy Considerations

- Feedback may contain PII (service names, user actions)
- Apply PII redaction rules before storing
- Aggregate patterns without exposing individual incidents
