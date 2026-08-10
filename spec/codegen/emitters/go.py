from __future__ import annotations

from typing import Any


def _go_list(values: list[str]) -> str:
    return ", ".join(f'"{value}"' for value in values)


def render_go_contract(contract: dict[str, Any]) -> str:
    enums = contract["enums"]
    top_level_fields = contract["allowed_top_level_fields"]
    collector_statuses = contract["collector_statuses"]
    collector_statuses_text = ", ".join(collector_statuses)
    return f'''package contract

import (
\t"bufio"
\t"bytes"
\t"encoding/json"
\t"fmt"
\t"regexp"
\t"strings"
)

const (
\tLOZASpecVersion      = "{contract["spec_version"]}"
\tLOZAEventVersion     = "{contract["event_version"]}"
\tLOZAIngestAPIVersion = "{contract["api_version"]}"
\tMaxEventBytes        = {contract["limits"]["max_event_size_bytes"]}
)

var rfc3339Pattern = regexp.MustCompile(`^(\\d{{4}}-\\d{{2}}-\\d{{2}}T\\d{{2}}:\\d{{2}}:\\d{{2}}(?:\\.\\d+)?(?:Z|[+-]\\d{{2}}:\\d{{2}}))$`)

type SourceInfo struct {{
\tSDK     string `json:"sdk"`
\tVersion string `json:"version"`
\tService string `json:"service"`
}}

type IngestEnvelope struct {{
\tAPIVersion string            `json:"api_version"`
\tSource     SourceInfo        `json:"source"`
\tEvents     []json.RawMessage `json:"events"`
}}

type EventAck struct {{
\tEventID   string `json:"event_id,omitempty"`
\tStatus    string `json:"status,omitempty"`
\tRetryable bool   `json:"retryable,omitempty"`
\tReason    string `json:"reason,omitempty"`
\tMessage   string `json:"message,omitempty"`
}}

type EventError struct {{
\tCode      string `json:"code,omitempty"`
\tMessage   string `json:"message,omitempty"`
\tRetryable bool   `json:"retryable,omitempty"`
\tField     string `json:"field,omitempty"`
\tEventID   string `json:"event_id,omitempty"`
}}

type CollectorResponse struct {{
\tRequestID string       `json:"request_id,omitempty"`
\tStatus    string       `json:"status,omitempty"`
\tAccepted  int          `json:"accepted,omitempty"`
\tRejected  int          `json:"rejected,omitempty"`
\tInvalid   int          `json:"invalid,omitempty"`
\tDeduped   int          `json:"deduped,omitempty"`
\tReason    string       `json:"reason,omitempty"`
\tError     string       `json:"error,omitempty"`
\tAcks      []EventAck   `json:"acks,omitempty"`
\tErrors    []EventError `json:"errors,omitempty"`
}}

type ValidationError struct {{
\tField     string `json:"field"`
\tCode      string `json:"code"`
\tMessage   string `json:"message"`
\tEventID   string `json:"event_id,omitempty"`
\tRetryable bool   `json:"retryable,omitempty"`
}}

type ValidationErrors []ValidationError

func (errs ValidationErrors) Error() string {{
\tif len(errs) == 0 {{
\t\treturn ""
\t}}
\treturn errs[0].Message
}}

func set(values ...string) map[string]struct{{}} {{
\tout := make(map[string]struct{{}}, len(values))
\tfor _, value := range values {{
\t\tout[value] = struct{{}}{{}}
\t}}
\treturn out
}}

var AllowedKinds = set({_go_list(enums["kinds"])})
var AllowedLevels = set({_go_list(enums["levels"])})
var AllowedOutcomes = set({_go_list(enums["outcomes"])})
var AllowedPartialReasons = set({_go_list(enums["partial_reasons"])})
var AllowedEventStates = set({_go_list(enums["event_states"])})
var AllowedSourceSDKs = set({_go_list(enums["source_sdks"])})
var AllowedTopLevelFields = set({_go_list(top_level_fields)})
var AllowedCollectorStatuses = set({_go_list(collector_statuses)})
var CanonicalFieldSet = AllowedTopLevelFields

func IsCanonical(key string) bool {{
\t_, ok := CanonicalFieldSet[key]
\treturn ok
}}

func LooksLikeLozaEventMap(payload map[string]any) bool {{
\tif payload == nil {{
\t\treturn false
\t}}
\t_, hasSchema := payload["schema_version"]
\t_, hasVersion := payload["event_version"]
\t_, hasEvent := payload["event"]
\t_, hasAlias := payload["event_type"]
\treturn hasSchema || hasVersion || hasEvent || hasAlias
}}

func NormalizeEventAliasesCopyMap(payload map[string]any) (map[string]any, bool) {{
\tcopyPayload := make(map[string]any, len(payload))
\tfor key, value := range payload {{
\t\tcopyPayload[key] = value
\t}}
\tif event, ok := copyPayload["event"].(string); ok && strings.TrimSpace(event) != "" {{
\t\tif _, hasAlias := copyPayload["event_type"]; hasAlias {{
\t\t\tdelete(copyPayload, "event_type")
\t\t\treturn copyPayload, true
\t\t}}
\t\treturn copyPayload, false
\t}}
\tif alias, ok := copyPayload["event_type"].(string); ok && strings.TrimSpace(alias) != "" {{
\t\tcopyPayload["event"] = strings.TrimSpace(alias)
\t\tdelete(copyPayload, "event_type")
\t\treturn copyPayload, true
\t}}
\treturn copyPayload, false
}}

func NormalizeEventAliasesMap(payload map[string]any) bool {{
\tnormalized, changed := NormalizeEventAliasesCopyMap(payload)
\tfor key := range payload {{
\t\tdelete(payload, key)
\t}}
\tfor key, value := range normalized {{
\t\tpayload[key] = value
\t}}
\treturn changed
}}

func BuildIngestEnvelope(sdk, version, service string, events []json.RawMessage) IngestEnvelope {{
\tif strings.TrimSpace(service) == "" {{
\t\tservice = "unknown"
\t}}
\treturn IngestEnvelope{{APIVersion: LOZAIngestAPIVersion, Source: SourceInfo{{SDK: sdk, Version: version, Service: service}}, Events: events}}
}}

func MarshalIngestEnvelope(sdk, version, service string, events []json.RawMessage) ([]byte, error) {{
\treturn json.Marshal(BuildIngestEnvelope(sdk, version, service, events))
}}

func ParseCollectorResponse(raw []byte) (CollectorResponse, error) {{
\tvar payload CollectorResponse
\terr := json.Unmarshal(bytes.TrimSpace(raw), &payload)
\treturn payload, err
}}

func (r CollectorResponse) RetryableError() (bool, string) {{
\tfor _, item := range r.Errors {{
\t\tif item.Retryable {{
\t\t\treturn true, firstNonEmpty(item.Message, item.Code)
\t\t}}
\t}}
\tfor _, ack := range r.Acks {{
\t\tif ack.Retryable {{
\t\t\treturn true, firstNonEmpty(ack.Message, ack.Reason, ack.Status)
\t\t}}
\t}}
\treturn false, ""
}}

func (r CollectorResponse) PermanentFailure() (bool, string) {{
\tif r.Rejected <= 0 && r.Invalid <= 0 && r.Status != "partial" && r.Status != "rejected" && r.Status != "invalid" {{
\t\treturn false, ""
\t}}
\tfor _, ack := range r.Acks {{
\t\tif ack.Retryable {{
\t\t\tcontinue
\t\t}}
\t\tif ack.Status == "rejected" || ack.Status == "invalid" {{
\t\t\treturn true, firstNonEmpty(ack.Message, ack.Reason, ack.Status)
\t\t}}
\t}}
\tfor _, item := range r.Errors {{
\t\tif item.Retryable {{
\t\t\tcontinue
\t\t}}
\t\treturn true, firstNonEmpty(item.Message, item.Code)
\t}}
\treturn true, firstNonEmpty(r.Error, r.Reason, fmt.Sprintf("accepted=%d rejected=%d invalid=%d", r.Accepted, r.Rejected, r.Invalid))
}}

func ValidateFlexibleJSONBytes(raw []byte, strict bool) error {{
\ttrimmed := bytes.TrimSpace(raw)
\tif len(trimmed) == 0 {{
\t\treturn ValidationErrors{{{{Field: "payload", Code: "empty_payload", Message: "payload is empty"}}}}
\t}}
\tif len(trimmed) > MaxEventBytes && !bytes.Contains(trimmed, []byte(`"events"`)) && !bytes.Contains(trimmed, []byte("\\n")) {{
\t\treturn ValidationErrors{{{{Field: "payload", Code: "too_large", Message: fmt.Sprintf("payload exceeds max_event_size_bytes (%d > %d)", len(trimmed), MaxEventBytes)}}}}
\t}}
\tif json.Valid(trimmed) {{
\t\tvar payload any
\t\tif err := json.Unmarshal(trimmed, &payload); err != nil {{
\t\t\treturn err
\t\t}}
\t\treturn validateAny(payload, strict)
\t}}
\tscanner := bufio.NewScanner(bytes.NewReader(trimmed))
\tfor scanner.Scan() {{
\t\tline := bytes.TrimSpace(scanner.Bytes())
\t\tif len(line) == 0 {{
\t\t\tcontinue
\t\t}}
\t\tvar payload any
\t\tif err := json.Unmarshal(line, &payload); err != nil {{
\t\t\treturn err
\t\t}}
\t\tif err := validateAny(payload, strict); err != nil {{
\t\t\treturn err
\t\t}}
\t}}
\treturn nil
}}

func ValidateEventBytes(raw []byte, strict bool) error {{
\tvar payload map[string]any
\tif err := json.Unmarshal(bytes.TrimSpace(raw), &payload); err != nil {{
\t\treturn err
\t}}
\treturn ValidateEventMap(payload, strict)
}}

func ValidateCollectorResponseMap(payload map[string]any) error {{
\terrs := ValidationErrors{{}}
\trequireString(payload, "request_id", "missing_request_id", &errs)
\trequireString(payload, "status", "missing_status", &errs)
\t
\t// Validate status is one of the allowed values
\tif status, ok := payload["status"].(string); ok {{
\t\tif _, exists := AllowedCollectorStatuses[strings.TrimSpace(status)]; !exists {{
\t\t\terrs = append(errs, ValidationError{{Field: "status", Code: "invalid_status", Message: fmt.Sprintf("field \\"status\\" must be one of: {collector_statuses_text}; got %q", status)}})
\t\t}}
\t}}
\t
\toptionalInteger(payload, "accepted", &errs)
\toptionalInteger(payload, "rejected", &errs)
\toptionalInteger(payload, "invalid", &errs)
\toptionalInteger(payload, "deduped", &errs)
\tif len(errs) == 0 {{
\t\treturn nil
\t}}
\treturn errs
}}

func ValidateEventMap(payload map[string]any, strict bool) error {{
\terrs := ValidateEventMapDetailed(payload, strict)
\tif len(errs) == 0 {{
\t\treturn nil
\t}}
\treturn errs
}}

func ValidateEventMapDetailed(payload map[string]any, strict bool) ValidationErrors {{
\terrs := ValidationErrors{{}}
\t
\t// STRICT MODE: Reject aliases BEFORE normalization
\tif strict {{
\t\tif _, hasAlias := payload["event_type"]; hasAlias {{
\t\t\terrs = append(errs, ValidationError{{Field: "event_type", Code: "alias_not_allowed", Message: "field \\"event_type\\" is not allowed in strict mode; use \\"event\\" instead"}})
\t\t}}
\t}}
\t
\tnormalized, _ := NormalizeEventAliasesCopyMap(payload)
\t
\tif strict {{
\t\tfor key := range normalized {{
\t\t\tif _, ok := AllowedTopLevelFields[key]; !ok {{
\t\t\t\terrs = append(errs, ValidationError{{Field: key, Code: "unknown_strict_field", Message: fmt.Sprintf("field %q is not allowed by strict schema", key)}})
\t\t\t}}
\t\t}}
\t}}
\trequireEnum(normalized, "schema_version", set(LOZASpecVersion), "unsupported_schema_version", &errs)
\trequireEnum(normalized, "event_version", set(LOZAEventVersion), "unsupported_event_version", &errs)
\trequireString(normalized, "event_id", "missing_event_id", &errs)
\trequireTimestamp(normalized, "timestamp", &errs)
\trequireService(normalized, &errs)
\trequireString(normalized, "event", "missing_event", &errs)
\trequireEnum(normalized, "kind", AllowedKinds, "invalid_enum", &errs)
\toptionalEnum(normalized, "level", AllowedLevels, &errs)
\toptionalEnum(normalized, "outcome", AllowedOutcomes, &errs)
\toptionalEnum(normalized, "partial_reason", AllowedPartialReasons, &errs)
\toptionalEnum(normalized, "event_state", AllowedEventStates, &errs)
\toptionalInteger(normalized, "duration_ms", &errs)
\toptionalStatusCode(normalized, "status_code", &errs)
\treturn errs
}}

func requireString(payload map[string]any, field, code string, errs *ValidationErrors) {{
\tvalue, ok := payload[field].(string)
\tif !ok || strings.TrimSpace(value) == "" {{
\t\t*errs = append(*errs, ValidationError{{Field: field, Code: code, Message: fmt.Sprintf("field %q must be a non-empty string", field)}})
\t}}
}}

func requireEnum(payload map[string]any, field string, allowed map[string]struct{{}}, code string, errs *ValidationErrors) {{
\trequireString(payload, field, code, errs)
\tif value, ok := payload[field].(string); ok && strings.TrimSpace(value) != "" {{
\t\tif _, exists := allowed[strings.TrimSpace(value)]; !exists {{
\t\t\t*errs = append(*errs, ValidationError{{Field: field, Code: code, Message: fmt.Sprintf("field %q has unsupported value %q", field, value)}})
\t\t}}
\t}}
}}

func optionalEnum(payload map[string]any, field string, allowed map[string]struct{{}}, errs *ValidationErrors) {{
\tvalue, ok := payload[field]
\tif !ok {{
\t\treturn
\t}}
\ttext, ok := value.(string)
\tif !ok || strings.TrimSpace(text) == "" {{
\t\t*errs = append(*errs, ValidationError{{Field: field, Code: "invalid_type", Message: fmt.Sprintf("field %q must be a non-empty string when set", field)}})
\t\treturn
\t}}
\tif _, exists := allowed[strings.TrimSpace(text)]; !exists {{
\t\t*errs = append(*errs, ValidationError{{Field: field, Code: "invalid_enum", Message: fmt.Sprintf("field %q has unsupported value %q", field, text)}})
\t}}
}}

func optionalInteger(payload map[string]any, field string, errs *ValidationErrors) {{
\tvalue, ok := payload[field]
\tif !ok {{
\t\treturn
\t}}
\tswitch typed := value.(type) {{
\tcase float64:
\t\tif typed >= 0 && typed == float64(int64(typed)) {{
\t\t\treturn
\t\t}}
\tcase int, int32, int64, uint, uint32, uint64:
\t\treturn
\t}}
\t*errs = append(*errs, ValidationError{{Field: field, Code: "invalid_integer", Message: fmt.Sprintf("field %q must be a non-negative integer", field)}})
}}

func optionalStatusCode(payload map[string]any, field string, errs *ValidationErrors) {{
\tvalue, ok := payload[field]
\tif !ok {{
\t\treturn
\t}}
\tvar code int
\tswitch typed := value.(type) {{
\tcase float64:
\t\tcode = int(typed)
\tcase int:
\t\tcode = typed
\tcase int32:
\t\tcode = int(typed)
\tcase int64:
\t\tcode = int(typed)
\tdefault:
\t\t*errs = append(*errs, ValidationError{{Field: field, Code: "invalid_status_code", Message: fmt.Sprintf("field %q must be between 100 and 599", field)}})
\t\treturn
\t}}
\tif code < 100 || code > 599 {{
\t\t*errs = append(*errs, ValidationError{{Field: field, Code: "invalid_status_code", Message: fmt.Sprintf("field %q must be between 100 and 599", field)}})
\t}}
}}

func requireTimestamp(payload map[string]any, field string, errs *ValidationErrors) {{
\tvalue, ok := payload[field].(string)
\tif !ok || !rfc3339Pattern.MatchString(strings.TrimSpace(value)) {{
\t\t*errs = append(*errs, ValidationError{{Field: field, Code: "invalid_rfc3339", Message: fmt.Sprintf("field %q must be RFC3339", field)}})
\t}}
}}

func requireService(payload map[string]any, errs *ValidationErrors) {{
\tvalue, ok := payload["service"]
\tif !ok {{
\t\t*errs = append(*errs, ValidationError{{Field: "service", Code: "missing_service", Message: "field \\"service\\" must be a non-empty string or object with name"}})
\t\treturn
\t}}
\tif text, ok := value.(string); ok && strings.TrimSpace(text) != "" {{
\t\treturn
\t}}
\tif service, ok := value.(map[string]any); ok {{
\t\tif name, ok := service["name"].(string); ok && strings.TrimSpace(name) != "" {{
\t\t\treturn
\t\t}}
\t}}
\t*errs = append(*errs, ValidationError{{Field: "service", Code: "invalid_service", Message: "field \\"service\\" must be a non-empty string or object with name"}})
}}

func ValidateIngestEnvelopeMap(payload map[string]any, strict bool) error {{
\terrs := ValidationErrors{{}}
\t
\t// Validate required envelope fields
\trequireString(payload, "api_version", "missing_api_version", &errs)
\tif source, ok := payload["source"].(map[string]any); ok {{
\t\trequireString(source, "sdk", "missing_source_sdk", &errs)
\t\trequireString(source, "service", "missing_source_service", &errs)
\t}} else {{
\t\terrs = append(errs, ValidationError{{Field: "source", Code: "missing_source", Message: "field \\"source\\" must be an object with sdk and service"}})
\t}}
\t
\t// Validate events array
\tif events, ok := payload["events"].([]any); ok {{
\t\tfor i, item := range events {{
\t\t\tevent, ok := item.(map[string]any)
\t\t\tif !ok {{
\t\t\t\terrs = append(errs, ValidationError{{Field: fmt.Sprintf("events[%d]", i), Code: "invalid_event", Message: "event must be a JSON object"}})
\t\t\t\tcontinue
\t\t\t}}
\t\t\tif eventErr := ValidateEventMap(event, strict); eventErr != nil {{
\t\t\t\tif validationErrs, ok := eventErr.(ValidationErrors); ok {{
\t\t\t\t\tfor _, err := range validationErrs {{
\t\t\t\t\t\terr.Field = fmt.Sprintf("events[%d].%s", i, err.Field)
\t\t\t\t\t\terrs = append(errs, err)
\t\t\t\t\t}}
\t\t\t\t}} else {{
\t\t\t\t\terrs = append(errs, ValidationError{{Field: fmt.Sprintf("events[%d]", i), Code: "validation_error", Message: eventErr.Error()}})
\t\t\t\t}}
\t\t\t}}
\t\t}}
\t}} else {{
\t\terrs = append(errs, ValidationError{{Field: "events", Code: "missing_events", Message: "field \\"events\\" must be an array"}})
\t}}
\t
\tif len(errs) == 0 {{
\t\treturn nil
\t}}
\treturn errs
}}

func validateAny(payload any, strict bool) error {{
\tswitch typed := payload.(type) {{
\tcase map[string]any:
\t\tif _, ok := typed["events"]; ok {{
\t\t\treturn ValidateIngestEnvelopeMap(typed, strict)
\t\t}}
\t\treturn ValidateEventMap(typed, strict)
\tcase []any:
\t\tfor _, item := range typed {{
\t\t\tevent, ok := item.(map[string]any)
\t\t\tif !ok {{
\t\t\t\treturn ValidationErrors{{{{Field: "events", Code: "invalid_event", Message: "event array must contain JSON objects"}}}}
\t\t\t}}
\t\t\tif err := ValidateEventMap(event, strict); err != nil {{
\t\t\t\treturn err
\t\t\t}}
\t\t}}
\t\treturn nil
\tdefault:
\t\treturn ValidationErrors{{{{Field: "payload", Code: "invalid_payload", Message: "payload must be a JSON object, array, wrapper, or NDJSON"}}}}
\t}}
}}

func firstNonEmpty(values ...string) string {{
\tfor _, value := range values {{
\t\tif strings.TrimSpace(value) != "" {{
\t\t\treturn value
\t\t}}
\t}}
\treturn ""
}}
'''
