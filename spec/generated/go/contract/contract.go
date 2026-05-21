package contract

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	LOXASpecVersion      = "v1"
	LOXAEventVersion     = "v1"
	LOXAIngestAPIVersion = "v1"
	MaxEventBytes        = 65536
)


type SourceInfo struct {
	SDK     string `json:"sdk"`
	Version string `json:"version"`
	Service string `json:"service"`
}

type IngestEnvelope struct {
	APIVersion string            `json:"api_version"`
	Source     SourceInfo        `json:"source"`
	Events     []json.RawMessage `json:"events"`
}

type EventAck struct {
	EventID   string `json:"event_id,omitempty"`
	Status    string `json:"status,omitempty"`
	Retryable bool   `json:"retryable,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Message   string `json:"message,omitempty"`
}

type EventError struct {
	Code      string `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
	Retryable bool   `json:"retryable,omitempty"`
	Field     string `json:"field,omitempty"`
	EventID   string `json:"event_id,omitempty"`
}

type CollectorResponse struct {
	RequestID string       `json:"request_id,omitempty"`
	Status    string       `json:"status,omitempty"`
	Accepted  int          `json:"accepted,omitempty"`
	Rejected  int          `json:"rejected,omitempty"`
	Invalid   int          `json:"invalid,omitempty"`
	Deduped   int          `json:"deduped,omitempty"`
	Reason    string       `json:"reason,omitempty"`
	Error     string       `json:"error,omitempty"`
	Acks      []EventAck   `json:"acks,omitempty"`
	Errors    []EventError `json:"errors,omitempty"`
}

type ValidationError struct {
	Field     string `json:"field"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	EventID   string `json:"event_id,omitempty"`
	Retryable bool   `json:"retryable,omitempty"`
}

type ValidationErrors []ValidationError

func (errs ValidationErrors) Error() string {
	if len(errs) == 0 {
		return ""
	}
	return errs[0].Message
}

func set(values ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

var AllowedKinds = set("event", "http", "job", "queue", "cli", "cron", "log", "checkpoint")
var AllowedLevels = set("debug", "info", "warn", "error", "fatal")
var AllowedOutcomes = set("success", "error", "timeout", "cancelled", "rejected", "abandoned", "partial", "unknown")
var AllowedPartialReasons = set("not_finished", "process_exit", "timeout", "panic", "collector_unavailable")
var AllowedEventStates = set("created", "active", "finished", "emitting", "emitted", "failed_validation", "delivery_failed")
var AllowedSourceSDKs = set("loxa-cli", "loxa-go", "loxa-py", "loxa-rs")
var AllowedTopLevelFields = set("attrs", "checkpoints", "delivery_attempts", "deployment", "duration_ms", "environment", "error", "event", "event_id", "event_state", "event_version", "http", "kind", "level", "message", "method", "organization", "outcome", "partial", "partial_reason", "path", "pii", "request_id", "resource", "route", "schema_version", "service", "source", "span_id", "status_code", "tenant", "timestamp", "trace_id", "user", "version", "workspace")
var AllowedCollectorStatuses = set("accepted", "partial", "rejected", "invalid")
var CanonicalFieldSet = AllowedTopLevelFields

func IsCanonical(key string) bool {
	_, ok := CanonicalFieldSet[key]
	return ok
}

func LooksLikeLoxaEventMap(payload map[string]any) bool {
	if payload == nil {
		return false
	}
	_, hasSchema := payload["schema_version"]
	_, hasVersion := payload["event_version"]
	_, hasEvent := payload["event"]
	_, hasAlias := payload["event_type"]
	return hasSchema || hasVersion || hasEvent || hasAlias
}

func NormalizeEventAliasesCopyMap(payload map[string]any) (map[string]any, bool) {
	copyPayload := make(map[string]any, len(payload))
	for key, value := range payload {
		copyPayload[key] = value
	}
	if event, ok := copyPayload["event"].(string); ok && strings.TrimSpace(event) != "" {
		if _, hasAlias := copyPayload["event_type"]; hasAlias {
			delete(copyPayload, "event_type")
			return copyPayload, true
		}
		return copyPayload, false
	}
	if alias, ok := copyPayload["event_type"].(string); ok && strings.TrimSpace(alias) != "" {
		copyPayload["event"] = strings.TrimSpace(alias)
		delete(copyPayload, "event_type")
		return copyPayload, true
	}
	return copyPayload, false
}

func NormalizeEventAliasesMap(payload map[string]any) bool {
	normalized, changed := NormalizeEventAliasesCopyMap(payload)
	for key := range payload {
		delete(payload, key)
	}
	for key, value := range normalized {
		payload[key] = value
	}
	return changed
}

func BuildIngestEnvelope(sdk, version, service string, events []json.RawMessage) IngestEnvelope {
	if strings.TrimSpace(service) == "" {
		service = "unknown"
	}
	return IngestEnvelope{APIVersion: LOXAIngestAPIVersion, Source: SourceInfo{SDK: sdk, Version: version, Service: service}, Events: events}
}

func MarshalIngestEnvelope(sdk, version, service string, events []json.RawMessage) ([]byte, error) {
	return json.Marshal(BuildIngestEnvelope(sdk, version, service, events))
}

func ParseCollectorResponse(raw []byte) (CollectorResponse, error) {
	var payload CollectorResponse
	err := json.Unmarshal(bytes.TrimSpace(raw), &payload)
	return payload, err
}

func (r CollectorResponse) RetryableError() (bool, string) {
	for _, item := range r.Errors {
		if item.Retryable {
			return true, firstNonEmpty(item.Message, item.Code)
		}
	}
	for _, ack := range r.Acks {
		if ack.Retryable {
			return true, firstNonEmpty(ack.Message, ack.Reason, ack.Status)
		}
	}
	return false, ""
}

func (r CollectorResponse) PermanentFailure() (bool, string) {
	if r.Rejected <= 0 && r.Invalid <= 0 && r.Status != "partial" && r.Status != "rejected" && r.Status != "invalid" {
		return false, ""
	}
	for _, ack := range r.Acks {
		if ack.Retryable {
			continue
		}
		if ack.Status == "rejected" || ack.Status == "invalid" {
			return true, firstNonEmpty(ack.Message, ack.Reason, ack.Status)
		}
	}
	for _, item := range r.Errors {
		if item.Retryable {
			continue
		}
		return true, firstNonEmpty(item.Message, item.Code)
	}
	return true, firstNonEmpty(r.Error, r.Reason, fmt.Sprintf("accepted=%d rejected=%d invalid=%d", r.Accepted, r.Rejected, r.Invalid))
}

func ValidateFlexibleJSONBytes(raw []byte, strict bool) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return ValidationErrors{{Field: "payload", Code: "empty_payload", Message: "payload is empty"}}
	}
	if len(trimmed) > MaxEventBytes && !bytes.Contains(trimmed, []byte(`"events"`)) && !bytes.Contains(trimmed, []byte("\n")) {
		return ValidationErrors{{Field: "payload", Code: "too_large", Message: fmt.Sprintf("payload exceeds max_event_size_bytes (%d > %d)", len(trimmed), MaxEventBytes)}}
	}
	if json.Valid(trimmed) {
		var payload any
		if err := json.Unmarshal(trimmed, &payload); err != nil {
			return err
		}
		return validateAny(payload, strict)
	}
	scanner := bufio.NewScanner(bytes.NewReader(trimmed))
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var payload any
		if err := json.Unmarshal(line, &payload); err != nil {
			return err
		}
		if err := validateAny(payload, strict); err != nil {
			return err
		}
	}
	return nil
}

func ValidateEventBytes(raw []byte, strict bool) error {
	var payload map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(raw), &payload); err != nil {
		return err
	}
	return ValidateEventMap(payload, strict)
}

func ValidateCollectorResponseMap(payload map[string]any) error {
	errs := ValidationErrors{}
	requireString(payload, "request_id", "missing_request_id", &errs)
	requireString(payload, "status", "missing_status", &errs)
	
	// Validate status is one of the allowed values
	if status, ok := payload["status"].(string); ok {
		if _, exists := AllowedCollectorStatuses[strings.TrimSpace(status)]; !exists {
			errs = append(errs, ValidationError{Field: "status", Code: "invalid_status", Message: fmt.Sprintf("field \"status\" must be one of: accepted, partial, rejected, invalid; got %q", status)})
		}
	}
	
	optionalInteger(payload, "accepted", &errs)
	optionalInteger(payload, "rejected", &errs)
	optionalInteger(payload, "invalid", &errs)
	optionalInteger(payload, "deduped", &errs)
	if len(errs) == 0 {
		return nil
	}
	return errs
}

func ValidateEventMap(payload map[string]any, strict bool) error {
	errs := ValidateEventMapDetailed(payload, strict)
	if len(errs) == 0 {
		return nil
	}
	return errs
}

func ValidateEventMapDetailed(payload map[string]any, strict bool) ValidationErrors {
	errs := ValidationErrors{}
	
	// STRICT MODE: Reject aliases BEFORE normalization
	if strict {
		if _, hasAlias := payload["event_type"]; hasAlias {
			errs = append(errs, ValidationError{Field: "event_type", Code: "alias_not_allowed", Message: "field \"event_type\" is not allowed in strict mode; use \"event\" instead"})
		}
	}
	
	normalized, _ := NormalizeEventAliasesCopyMap(payload)
	
	if strict {
		for key := range normalized {
			if _, ok := AllowedTopLevelFields[key]; !ok {
				errs = append(errs, ValidationError{Field: key, Code: "unknown_strict_field", Message: fmt.Sprintf("field %q is not allowed by strict schema", key)})
			}
		}
	}
	requireEnum(normalized, "schema_version", set(LOXASpecVersion), "unsupported_schema_version", &errs)
	requireEnum(normalized, "event_version", set(LOXAEventVersion), "unsupported_event_version", &errs)
	requireString(normalized, "event_id", "missing_event_id", &errs)
	requireTimestamp(normalized, "timestamp", &errs)
	requireService(normalized, &errs)
	requireString(normalized, "event", "missing_event", &errs)
	requireEnum(normalized, "kind", AllowedKinds, "invalid_enum", &errs)
	optionalEnum(normalized, "level", AllowedLevels, &errs)
	optionalEnum(normalized, "outcome", AllowedOutcomes, &errs)
	optionalEnum(normalized, "partial_reason", AllowedPartialReasons, &errs)
	optionalEnum(normalized, "event_state", AllowedEventStates, &errs)
	optionalInteger(normalized, "duration_ms", &errs)
	optionalStatusCode(normalized, "status_code", &errs)
	return errs
}

func requireString(payload map[string]any, field, code string, errs *ValidationErrors) {
	value, ok := payload[field].(string)
	if !ok || strings.TrimSpace(value) == "" {
		*errs = append(*errs, ValidationError{Field: field, Code: code, Message: fmt.Sprintf("field %q must be a non-empty string", field)})
	}
}

func requireEnum(payload map[string]any, field string, allowed map[string]struct{}, code string, errs *ValidationErrors) {
	requireString(payload, field, code, errs)
	if value, ok := payload[field].(string); ok && strings.TrimSpace(value) != "" {
		if _, exists := allowed[strings.TrimSpace(value)]; !exists {
			*errs = append(*errs, ValidationError{Field: field, Code: code, Message: fmt.Sprintf("field %q has unsupported value %q", field, value)})
		}
	}
}

func optionalEnum(payload map[string]any, field string, allowed map[string]struct{}, errs *ValidationErrors) {
	value, ok := payload[field]
	if !ok {
		return
	}
	text, ok := value.(string)
	if !ok || strings.TrimSpace(text) == "" {
		*errs = append(*errs, ValidationError{Field: field, Code: "invalid_type", Message: fmt.Sprintf("field %q must be a non-empty string when set", field)})
		return
	}
	if _, exists := allowed[strings.TrimSpace(text)]; !exists {
		*errs = append(*errs, ValidationError{Field: field, Code: "invalid_enum", Message: fmt.Sprintf("field %q has unsupported value %q", field, text)})
	}
}

func optionalInteger(payload map[string]any, field string, errs *ValidationErrors) {
	value, ok := payload[field]
	if !ok {
		return
	}
	switch typed := value.(type) {
	case float64:
		if typed >= 0 && typed == float64(int64(typed)) {
			return
		}
	case int, int32, int64, uint, uint32, uint64:
		return
	}
	*errs = append(*errs, ValidationError{Field: field, Code: "invalid_integer", Message: fmt.Sprintf("field %q must be a non-negative integer", field)})
}

func optionalStatusCode(payload map[string]any, field string, errs *ValidationErrors) {
	value, ok := payload[field]
	if !ok {
		return
	}
	var code int
	switch typed := value.(type) {
	case float64:
		code = int(typed)
	case int:
		code = typed
	case int32:
		code = int(typed)
	case int64:
		code = int(typed)
	default:
		*errs = append(*errs, ValidationError{Field: field, Code: "invalid_status_code", Message: fmt.Sprintf("field %q must be between 100 and 599", field)})
		return
	}
	if code < 100 || code > 599 {
		*errs = append(*errs, ValidationError{Field: field, Code: "invalid_status_code", Message: fmt.Sprintf("field %q must be between 100 and 599", field)})
	}
}

func requireTimestamp(payload map[string]any, field string, errs *ValidationErrors) {
	value, ok := payload[field].(string)
	if !ok {
		*errs = append(*errs, ValidationError{Field: field, Code: "invalid_rfc3339", Message: fmt.Sprintf("field %q must be RFC3339", field)})
		return
	}
	if _, err := time.Parse(time.RFC3339, strings.TrimSpace(value)); err != nil {
		*errs = append(*errs, ValidationError{Field: field, Code: "invalid_rfc3339", Message: fmt.Sprintf("field %q must be RFC3339", field)})
	}
}

func requireService(payload map[string]any, errs *ValidationErrors) {
	value, ok := payload["service"]
	if !ok {
		*errs = append(*errs, ValidationError{Field: "service", Code: "missing_service", Message: "field \"service\" must be a non-empty string or object with name"})
		return
	}
	if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
		return
	}
	if service, ok := value.(map[string]any); ok {
		if name, ok := service["name"].(string); ok && strings.TrimSpace(name) != "" {
			return
		}
	}
	*errs = append(*errs, ValidationError{Field: "service", Code: "invalid_service", Message: "field \"service\" must be a non-empty string or object with name"})
}

func ValidateIngestEnvelopeMap(payload map[string]any, strict bool) error {
	errs := ValidationErrors{}
	
	// Validate required envelope fields
	requireString(payload, "api_version", "missing_api_version", &errs)
	if source, ok := payload["source"].(map[string]any); ok {
		requireString(source, "sdk", "missing_source_sdk", &errs)
		requireString(source, "service", "missing_source_service", &errs)
	} else {
		errs = append(errs, ValidationError{Field: "source", Code: "missing_source", Message: "field \"source\" must be an object with sdk and service"})
	}
	
	// Validate events array
	if events, ok := payload["events"].([]any); ok {
		for i, item := range events {
			event, ok := item.(map[string]any)
			if !ok {
				errs = append(errs, ValidationError{Field: fmt.Sprintf("events[%d]", i), Code: "invalid_event", Message: "event must be a JSON object"})
				continue
			}
			if eventErr := ValidateEventMap(event, strict); eventErr != nil {
				if validationErrs, ok := eventErr.(ValidationErrors); ok {
					for _, err := range validationErrs {
						err.Field = fmt.Sprintf("events[%d].%s", i, err.Field)
						errs = append(errs, err)
					}
				} else {
					errs = append(errs, ValidationError{Field: fmt.Sprintf("events[%d]", i), Code: "validation_error", Message: eventErr.Error()})
				}
			}
		}
	} else {
		errs = append(errs, ValidationError{Field: "events", Code: "missing_events", Message: "field \"events\" must be an array"})
	}
	
	if len(errs) == 0 {
		return nil
	}
	return errs
}

func validateAny(payload any, strict bool) error {
	switch typed := payload.(type) {
	case map[string]any:
		if _, ok := typed["events"]; ok {
			return ValidateIngestEnvelopeMap(typed, strict)
		}
		return ValidateEventMap(typed, strict)
	case []any:
		for _, item := range typed {
			event, ok := item.(map[string]any)
			if !ok {
				return ValidationErrors{{Field: "events", Code: "invalid_event", Message: "event array must contain JSON objects"}}
			}
			if err := ValidateEventMap(event, strict); err != nil {
				return err
			}
		}
		return nil
	default:
		return ValidationErrors{{Field: "payload", Code: "invalid_payload", Message: "payload must be a JSON object, array, wrapper, or NDJSON"}}
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
