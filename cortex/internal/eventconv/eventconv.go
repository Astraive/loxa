package eventconv

import (
	"fmt"
	"strings"
	"time"

	speccontract "github.com/astraive/loxa-spec/generated/go/contract"
	"github.com/astraive/loxa/loxa-cortex/internal/models"
)

func FromRawMap(raw map[string]any, defaultProvenance string) (*models.Event, error) {
	event := &models.Event{}
	if speccontract.LooksLikeLoxaEventMap(raw) {
		speccontract.NormalizeEventAliasesMap(raw)
		if err := speccontract.ValidateEventMap(raw, false); err != nil {
			return nil, fmt.Errorf("loxa event contract validation failed: %w", err)
		}
	}

	if id, ok := raw["id"].(string); ok {
		event.ID = id
	}

	if ts, ok := raw["timestamp"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
			event.Timestamp = t
		} else if t, err := time.Parse(time.RFC3339, ts); err == nil {
			event.Timestamp = t
		} else if t, err := time.Parse("2006-01-02T15:04:05Z07:00", ts); err == nil {
			event.Timestamp = t
		} else if t, err := time.Parse("2006-01-02T15:04:05", ts); err == nil {
			event.Timestamp = t
		}
	} else if ts, ok := raw["timestamp"].(float64); ok {
		event.Timestamp = time.Unix(int64(ts), 0).UTC()
	}

	if kind, ok := raw["kind"].(string); ok {
		event.Kind = normalizeEventKind(kind)
	}

	if service, ok := raw["service"].(string); ok {
		event.Service = service
	}

	if traceID, ok := raw["trace_id"].(string); ok {
		event.TraceID = traceID
	}

	if incidentID, ok := raw["incident_id"].(string); ok {
		event.IncidentID = incidentID
	}

	if provenance, ok := raw["provenance"].(string); ok {
		event.Provenance = normalizeProvenance(provenance, defaultProvenance)
	} else {
		event.Provenance = normalizeProvenance(defaultProvenance, "loxa")
	}

	rawCopy := make(map[string]any)
	for k, v := range raw {
		switch k {
		case "id", "timestamp", "kind", "service", "trace_id", "incident_id", "provenance":
		default:
			rawCopy[k] = v
		}
	}
	event.Raw = rawCopy

	if err := event.Validate(); err != nil {
		return nil, fmt.Errorf("event validation failed: %w", err)
	}
	return event, nil
}

func normalizeEventKind(kind string) models.EventKind {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "deploy", "deployment":
		return models.EventKindDeploy
	case "metric", "metrics":
		return models.EventKindMetric
	case "trace", "span":
		return models.EventKindTrace
	case "topology":
		return models.EventKindTopology
	case "incident_signal":
		return models.EventKindIncidentSignal
	case "remediation":
		return models.EventKindRemediation
	case "otel_log":
		return models.EventKindOTELLog
	case "otel_span":
		return models.EventKindOTELSpan
	case "collector_event":
		return models.EventKindCollectorEvent
	case "log":
		return models.EventKindLog
	case "http", "job", "queue", "cli", "cron", "event", "checkpoint", "loxa_event":
		return models.EventKindLoxaEvent
	default:
		return models.EventKindCollectorEvent
	}
}

func normalizeProvenance(value, fallback string) string {
	switch {
	case strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "collector"):
		return "collector"
	case strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "otlp"):
		return "otlp"
	case strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "jsonl"):
		return "jsonl"
	case strings.TrimSpace(value) == "":
		if strings.TrimSpace(fallback) == "" {
			return "loxa"
		}
		return normalizeProvenance(fallback, "loxa")
	default:
		return "loxa"
	}
}

func NormalizeEventKind(kind string) models.EventKind {
	return normalizeEventKind(kind)
}

func NormalizeProvenance(value string) string {
	return normalizeProvenance(value, "loxa")
}
