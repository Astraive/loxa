package models

import (
	"errors"
	"time"
)

type SymptomType string

const (
	SymptomTypeLatencySpike    SymptomType = "latency_spike"
	SymptomTypeErrorRate    SymptomType = "error_rate"
	SymptomTypeTimeout      SymptomType = "timeout"
	SymptomTypeMemoryLeak   SymptomType = "memory_leak"
	SymptomTypeCPUSpike    SymptomType = "cpu_spike"
	SymptomTypeDeploymentFail SymptomType = "deployment_fail"
)

type Symptom struct {
	Type        SymptomType `json:"type"`
	Service    string    `json:"service"`
	Metric    string    `json:"metric,omitempty"`
	Threshold float64   `json:"threshold,omitempty"`
	Observed  float64   `json:"observed,omitempty"`
	Description string  `json:"description"`
}

type IncidentSignal struct {
	SignalID   string                 `json:"signal_id" db:"signal_id"`
	Timestamp  time.Time              `json:"timestamp" db:"timestamp"`
	Service    string                 `json:"service" db:"service"`
	SignalType string                 `json:"signal_type" db:"signal_type"`
	Severity   string                 `json:"severity" db:"severity"`
	Attributes map[string]interface{} `json:"attributes" db:"attributes"`
}

type Incident struct {
	ID              string    `json:"id" db:"id"`
	Timestamp       time.Time `json:"timestamp" db:"timestamp"`
	SignatureID     string   `json:"signature_id,omitempty" db:"signature_id"`
	Status         string   `json:"status" db:"status"`
	Severity       string   `json:"severity" db:"severity"`
	PrimaryService string   `json:"primary_service" db:"primary_service"`
	AffectedServices []string `json:"affected_services" db:"affected_services"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty" db:"resolved_at"`
}

func (i *Incident) Validate() error {
	if i.ID == "" {
		return errors.New("incident ID cannot be empty")
	}
	validStatuses := map[string]bool{"active": true, "resolved": true, "unknown": true}
	if !validStatuses[i.Status] {
		return errors.New("invalid incident status")
	}
	validSeverities := map[string]bool{"critical": true, "high": true, "medium": true, "low": true}
	if !validSeverities[i.Severity] {
		return errors.New("invalid incident severity")
	}
	return nil
}