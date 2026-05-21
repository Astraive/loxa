package models

import (
	"errors"
	"time"
)

// OutcomeCategory derives the category from an HTTP-style outcome code.
func OutcomeCategory(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "success"
	case code >= 300 && code < 400:
		return "partial"
	default:
		return "failed"
	}
}

// IsValidOutcomeCode checks if the code is a valid HTTP-style status code.
func IsValidOutcomeCode(code int) bool {
	return code >= 100 && code < 600
}

type Remediation struct {
	RemediationID string                 `json:"remediation_id" db:"remediation_id"`
	IncidentID  string                 `json:"incident_id" db:"incident_id"`
	SignatureID string                 `json:"signature_id,omitempty" db:"signature_id"`
	Action     string                 `json:"action" db:"action"`
	Timestamp  time.Time              `json:"timestamp" db:"timestamp"`
	Operator  string                 `json:"operator,omitempty" db:"operator"`
	Attributes map[string]interface{} `json:"attributes" db:"attributes"`
}

type RemediationFeedback struct {
	FeedbackID      string    `json:"feedback_id" db:"feedback_id"`
	RemediationID   string    `json:"remediation_id" db:"remediation_id"`
	IncidentID      string    `json:"incident_id" db:"incident_id"`
	OutcomeCode     int       `json:"outcome_code" db:"outcome_code"`
	OutcomeCategory string    `json:"outcome_category" db:"outcome_category"`
	TimeToResolve   int64     `json:"time_to_resolve_seconds" db:"time_to_resolve_seconds"`
	Timestamp       time.Time `json:"timestamp" db:"timestamp"`
	Notes           string    `json:"notes,omitempty" db:"notes"`
}

func (f *RemediationFeedback) Validate() error {
	if !IsValidOutcomeCode(f.OutcomeCode) {
		return errors.New("invalid outcome code: must be 100-599")
	}
	return nil
}

type RemediationStats struct {
	Action           string  `json:"action"`
	SuccessRate     float64 `json:"success_rate"`
	TotalAttempts   int     `json:"total_attempts"`
	SuccessfulCount int     `json:"successful_count"`
	AvgTimeToResolve int64   `json:"avg_time_to_resolve_seconds"`
}