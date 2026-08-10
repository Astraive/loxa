package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	speccontract "github.com/astraive/loza/spec/generated/go/contract"
)

const (
	LOZA_SPEC_VERSION       = speccontract.LOZASpecVersion
	LOZA_INGEST_API_VERSION = speccontract.LOZAIngestAPIVersion
	LOZA_EVENT_VERSION      = speccontract.LOZAEventVersion
)

// ValidateIngestEnvelopeBytes validates a runtime envelope payload against the
// generated contract before it is sent to the collector.
func ValidateIngestEnvelopeBytes(raw []byte, strict bool) error {
	var payload map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(raw), &payload); err != nil {
		return err
	}
	return validateIngestEnvelopeShape(payload)
}

// ValidateEventBytes validates a single event JSON payload against the spec contract.
func ValidateEventBytes(raw []byte, strict bool) error {
	var payload map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(raw), &payload); err != nil {
		return fmt.Errorf("loza: event must be valid JSON: %w", err)
	}
	if strict {
		eventName, ok := payload["event"].(string)
		if !ok || strings.TrimSpace(eventName) == "" {
			return fmt.Errorf("loza: event must include a non-empty 'event' field")
		}
	}
	return nil
}

func validateIngestEnvelopeShape(payload map[string]any) error {
	if version, ok := payload["api_version"].(string); !ok || strings.TrimSpace(version) != LOZA_INGEST_API_VERSION {
		return fmt.Errorf("collector envelope must include api_version %q", LOZA_INGEST_API_VERSION)
	}

	source, ok := payload["source"].(map[string]any)
	if !ok {
		return fmt.Errorf("collector envelope must include a source object")
	}
	for _, key := range []string{"sdk", "version", "service"} {
		value, ok := source[key].(string)
		if !ok || strings.TrimSpace(value) == "" {
			return fmt.Errorf("collector envelope source.%s must be a non-empty string", key)
		}
	}

	events, ok := payload["events"].([]any)
	if !ok {
		return fmt.Errorf("collector envelope must include an events array")
	}
	if len(events) == 0 {
		return fmt.Errorf("collector envelope must include at least one event")
	}
	for idx, event := range events {
		if _, ok := event.(map[string]any); !ok {
			return fmt.Errorf("collector envelope events[%d] must be JSON objects", idx)
		}
	}
	return nil
}
