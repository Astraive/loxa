package models

import (
	"time"
)

type TemporalPattern struct {
	TriggerToSymptom int64  `json:"trigger_to_symptom_ms"`
	SymptomDuration  int64  `json:"symptom_duration_ms"`
	PropagationSpeed string `json:"propagation_speed"`
}

type SignatureUpdate struct {
	AddRemediation       []string `json:"add_remediation,omitempty"`
	IncrementOccurrence bool   `json:"increment_occurrence"`
	UpdateResolutionTime int64 `json:"update_resolution_time,omitempty"`
}

type IncidentSignature struct {
	SignatureID        string     `json:"signature_id" db:"signature_id"`
	Shape              string     `json:"shape" db:"shape"`
	ServiceRoles       []string   `json:"service_roles" db:"service_roles"`
	Symptoms           []SymptomType `json:"symptoms" db:"symptoms"`
	TemporalPattern    TemporalPattern `json:"temporal_pattern" db:"temporal_pattern"`
	Remediation        []string   `json:"remediation" db:"remediation"`
	FeatureVector      []float64  `json:"feature_vector" db:"feature_vector"`
	FeatureWeights     []float64  `json:"feature_weights,omitempty" db:"feature_weights"`
	OccurrenceCount    int        `json:"occurrence_count" db:"occurrence_count"`
	AvgResolutionTime  int64      `json:"avg_resolution_time_seconds" db:"avg_resolution_time_seconds"`
	Version            int        `json:"version" db:"version"`
	ParentSignatureID  string     `json:"parent_signature_id,omitempty" db:"parent_signature_id"`
	DecayFactor        float64    `json:"decay_factor" db:"decay_factor"`
	LastMatchedAt      *time.Time `json:"last_matched_at,omitempty" db:"last_matched_at"`
	BehavioralHash     string     `json:"behavioral_hash" db:"behavioral_hash"`
	CreatedAt          time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at" db:"updated_at"`
}

type SimilarIncident struct {
	IncidentID       string  `json:"incident_id"`
	Timestamp        time.Time `json:"timestamp"`
	Similarity      float64 `json:"similarity"`
	Shape            string  `json:"shape"`
	Resolution       string  `json:"resolution"`
	ResolutionTime  int64   `json:"resolution_time_seconds"`
	SuccessRate     float64 `json:"success_rate"`
}

type SalienceScore struct {
	EventType   string    `json:"event_type" db:"event_type"`
	Score       float64   `json:"score" db:"score"`
	SampleCount int       `json:"sample_count" db:"sample_count"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}