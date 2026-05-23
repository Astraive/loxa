package eventconv

import (
	"fmt"
	"strings"
	"time"

	speccontract "github.com/astraive/loxa/spec/generated/go/contract"
	"github.com/astraive/loxa/loxa-cortex/internal/models"
)

func FromRawMap(raw map[string]any, defaultProvenance string) (*models.Event, error) {
	event := &models.Event{}

	// Try spec contract validation if available
	if speccontract.LooksLikeLoxaEventMap(raw) {
		speccontract.NormalizeEventAliasesMap(raw)
		if err := speccontract.ValidateEventMap(raw, false); err != nil {
			return nil, fmt.Errorf("loxa event contract validation failed: %w", err)
		}
	}

	// Core identifiers
	event.ID = getString(raw, "id")
	event.EventID = getString(raw, "event_id")

	// Timestamp parsing
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

	// Schema & versioning
	event.SchemaVersion = getString(raw, "schema_version")
	event.EventVersion = getString(raw, "event_version")
	event.Version = getString(raw, "version")

	// Event fields
	event.Event = getString(raw, "event")
	if kind, ok := raw["kind"].(string); ok {
		event.Kind = normalizeEventKind(kind)
	}
	event.Service = getString(raw, "service")
	event.Environment = getString(raw, "environment")
	event.Release = getString(raw, "release")
	event.Level = getString(raw, "level")
	event.Outcome = getString(raw, "outcome")

	// Duration
	if dur, ok := getFloat(raw, "duration_ms"); ok {
		event.DurationMs = dur
	}

	// Trace correlation
	event.TraceID = getString(raw, "trace_id")
	event.SpanID = getString(raw, "span_id")
	event.TraceFlags = getString(raw, "trace_flags")
	event.RequestID = getString(raw, "request_id")

	// Incident
	event.IncidentID = getString(raw, "incident_id")

	// HTTP context
	if httpRaw, ok := raw["http"].(map[string]any); ok {
		http := &models.HttpContext{}
		http.Method = getString(httpRaw, "method")
		http.Path = getString(httpRaw, "path")
		http.Route = getString(httpRaw, "route")
		if sc, ok := getFloat(httpRaw, "status_code"); ok {
			http.StatusCode = int(sc)
		}
		http.ClientIP = getString(httpRaw, "client_ip")
		http.UserAgent = getString(httpRaw, "user_agent")
		event.HTTP = http
	}

	// User context
	if userRaw, ok := raw["user"].(map[string]any); ok {
		event.User = userRaw
	}

	// Tenant context
	if tenantRaw, ok := raw["tenant"].(map[string]any); ok {
		event.Tenant = tenantRaw
	}

	// Structured attributes
	if attrsRaw, ok := raw["attrs"].(map[string]any); ok {
		event.Attrs = attrsRaw
	} else {
		// Fallback: extract top-level attrs from remaining fields after removing reserved
		event.Attrs = make(map[string]any)
	}

	// Error context
	if errRaw, ok := raw["error"].(map[string]any); ok {
		errInfo := &models.EventError{}
		errInfo.Type = getString(errRaw, "type")
		errInfo.Message = getString(errRaw, "message")
		errInfo.Code = getString(errRaw, "code")
		errInfo.Cause = getString(errRaw, "cause")
		errInfo.Stack = getString(errRaw, "stack")
		event.Error = errInfo
	}

	// Lifecycle: Checkpoints
	if cps, ok := raw["checkpoints"].([]any); ok {
		for _, cpRaw := range cps {
			if cpMap, ok := cpRaw.(map[string]any); ok {
				cp := &models.EventCheckpoint{
					Name:  getString(cpMap, "name"),
					AtMs:  getInt64(cpMap, "at_ms"),
				}
				if a, ok := cpMap["attrs"].(map[string]any); ok {
					cp.Attrs = a
				}
				event.Checkpoints = append(event.Checkpoints, cp)
			}
		}
	}

	// Lifecycle: Processes
	if procs, ok := raw["processes"].([]any); ok {
		for i, procRaw := range procs {
			if procMap, ok := procRaw.(map[string]any); ok {
				p := &models.EventProcess{
					Step:        i + 1,
					Name:        getString(procMap, "name"),
					StatusCode:  int(getInt64(procMap, "status_code")),
					StartedAtMs: getInt64(procMap, "started_at_ms"),
					EndedAtMs:   getInt64(procMap, "ended_at_ms"),
					DurationMs:  getInt64(procMap, "duration_ms"),
					Outcome:     getString(procMap, "outcome"),
				}
				// Compute duration if not present
				if p.DurationMs == 0 && p.EndedAtMs > p.StartedAtMs {
					p.DurationMs = p.EndedAtMs - p.StartedAtMs
				}
				if a, ok := procMap["attrs"].(map[string]any); ok {
					p.Attrs = a
				}
				if errRaw, ok := procMap["error"].(map[string]any); ok {
					p.Error = extractError(errRaw)
				}
				event.Processes = append(event.Processes, p)
			}
		}
	}

	// Lifecycle: Groups
	if grps, ok := raw["groups"].([]any); ok {
		for _, grpRaw := range grps {
			if grpMap, ok := grpRaw.(map[string]any); ok {
				g := &models.EventGroup{
					Name:        getString(grpMap, "name"),
					StatusCode:  int(getInt64(grpMap, "status_code")),
					StartedAtMs: getInt64(grpMap, "started_at_ms"),
					EndedAtMs:   getInt64(grpMap, "ended_at_ms"),
					DurationMs:  getInt64(grpMap, "duration_ms"),
					Outcome:     getString(grpMap, "outcome"),
				}
				if g.DurationMs == 0 && g.EndedAtMs > g.StartedAtMs {
					g.DurationMs = g.EndedAtMs - g.StartedAtMs
				}
				if a, ok := grpMap["attrs"].(map[string]any); ok {
					g.Attrs = a
				}
				if errRaw, ok := grpMap["error"].(map[string]any); ok {
					g.Error = extractError(errRaw)
				}
				event.Groups = append(event.Groups, g)
			}
		}
	}

	// Lifecycle: Timers
	if timers, ok := raw["timers"].([]any); ok {
		for _, timerRaw := range timers {
			if timerMap, ok := timerRaw.(map[string]any); ok {
				t := &models.EventTimer{
					Name:       getString(timerMap, "name"),
					DurationMs: getInt64(timerMap, "duration_ms"),
					StatusCode: int(getInt64(timerMap, "status_code")),
				}
				if a, ok := timerMap["attrs"].(map[string]any); ok {
					t.Attrs = a
				}
				event.Timers = append(event.Timers, t)
			}
		}
	}

	// Lifecycle: Links
	if links, ok := raw["links"].([]any); ok {
		for _, linkRaw := range links {
			if linkMap, ok := linkRaw.(map[string]any); ok {
				l := &models.EventLink{
					Type:   getString(linkMap, "type"),
					Target: getString(linkMap, "target"),
				}
				if a, ok := linkMap["attrs"].(map[string]any); ok {
					l.Attrs = a
				}
				event.Links = append(event.Links, l)
			}
		}
	}

	// SDK metadata
	event.SDKName = getString(raw, "sdk_name")
	event.SDKVersion = getString(raw, "sdk_version")
	event.SDKLanguage = getString(raw, "sdk_language")

	// Provenance
	if provenance, ok := raw["provenance"].(string); ok {
		event.Provenance = normalizeProvenance(provenance, defaultProvenance)
	} else {
		event.Provenance = normalizeProvenance(defaultProvenance, "loxa")
	}

	// Preserve raw payload (everything except lifecycle fields that are extracted)
	rawCopy := make(map[string]any)
	for k, v := range raw {
		switch k {
		case "id", "event_id", "timestamp", "schema_version", "event_version", "version",
			"event", "kind", "service", "environment", "release", "level", "outcome", "duration_ms",
			"trace_id", "span_id", "trace_flags", "request_id",
			"http", "user", "tenant", "attrs", "error",
			"checkpoints", "processes", "groups", "timers", "links",
			"sdk_name", "sdk_version", "sdk_language",
			"provenance", "incident_id":
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

// getString safely extracts a string from a map
func getString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// getInt64 safely extracts an int64 from a map (handles float64 JSON unmarshaling)
func getInt64(m map[string]any, key string) int64 {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case float64:
			return int64(val)
		case int64:
			return val
		case int:
			return int64(val)
		case string:
			i, _ := time.ParseDuration(val)
			return i.Milliseconds()
		}
	}
	return 0
}

// getFloat safely extracts a float64 from a map
func getFloat(m map[string]any, key string) (float64, bool) {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case float64:
			return val, true
		case int:
			return float64(val), true
		case int64:
			return float64(val), true
		case string:
			i, err := time.ParseDuration(val)
			if err == nil {
				return float64(i.Milliseconds()), true
			}
			return 0, false
		}
	}
	return 0, false
}

// extractError creates an EventError from a raw map
func extractError(m map[string]any) *models.EventError {
	return &models.EventError{
		Type:    getString(m, "type"),
		Message: getString(m, "message"),
		Code:    getString(m, "code"),
		Cause:   getString(m, "cause"),
		Stack:   getString(m, "stack"),
	}
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
