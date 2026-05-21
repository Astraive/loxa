package otlpconv

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	collectorlogsv1 "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	logsv1 "go.opentelemetry.io/proto/otlp/logs/v1"
)

type Config struct {
	SchemaVersion  string
	EventVersion   string
	DefaultService string
}

func ConvertExportLogsRequest(req *collectorlogsv1.ExportLogsServiceRequest, cfg Config) ([][]byte, error) {
	if req == nil {
		return nil, nil
	}
	return convertResourceLogs(req.ResourceLogs, cfg)
}

func convertResourceLogs(resourceLogs []*logsv1.ResourceLogs, cfg Config) ([][]byte, error) {
	out := make([][]byte, 0)
	for _, resourceLog := range resourceLogs {
		resourceAttrs := kvToMap(nil, resourceLog.GetResource().GetAttributes())
		service := stringValue(resourceAttrs["service.name"])
		environment := stringValue(resourceAttrs["deployment.environment"])
		for _, scopeLogs := range resourceLog.GetScopeLogs() {
			for _, record := range scopeLogs.GetLogRecords() {
				event := map[string]any{
					"schema_version": cfg.SchemaVersion,
					"event_version":  cfg.EventVersion,
					"timestamp":      timestampFromNano(record.GetTimeUnixNano(), record.GetObservedTimeUnixNano()),
					"event_id":       fmt.Sprintf("otlp_%d", record.GetTimeUnixNano()),
					"service":        fallbackString(service, cfg.DefaultService, "otlp"),
					"event":          bodyToEventName(record.GetBody()),
					"kind":           "log",
					"level":          strings.ToLower(strings.TrimSpace(record.GetSeverityText())),
					"attrs":          kvToMap(resourceAttrs, record.GetAttributes()),
					"source": map[string]any{
						"sdk": "otlp",
					},
				}
				if environment != "" {
					event["deployment"] = map[string]any{"environment": environment}
				}
				raw, err := json.Marshal(event)
				if err != nil {
					return nil, err
				}
				out = append(out, raw)
			}
		}
	}
	return out, nil
}

func kvToMap(base map[string]any, attrs []*commonv1.KeyValue) map[string]any {
	out := map[string]any{}
	for k, v := range base {
		out[k] = v
	}
	for _, attr := range attrs {
		out[attr.GetKey()] = anyValue(attr.GetValue())
	}
	return out
}

func anyValue(v *commonv1.AnyValue) any {
	if v == nil {
		return ""
	}
	switch value := v.GetValue().(type) {
	case *commonv1.AnyValue_StringValue:
		return value.StringValue
	case *commonv1.AnyValue_BoolValue:
		return value.BoolValue
	case *commonv1.AnyValue_IntValue:
		return value.IntValue
	case *commonv1.AnyValue_DoubleValue:
		return value.DoubleValue
	case *commonv1.AnyValue_ArrayValue:
		out := make([]any, 0, len(value.ArrayValue.GetValues()))
		for _, item := range value.ArrayValue.GetValues() {
			out = append(out, anyValue(item))
		}
		return out
	case *commonv1.AnyValue_KvlistValue:
		m := map[string]any{}
		for _, item := range value.KvlistValue.GetValues() {
			m[item.GetKey()] = anyValue(item.GetValue())
		}
		return m
	case *commonv1.AnyValue_BytesValue:
		return string(value.BytesValue)
	default:
		return ""
	}
}

func timestampFromNano(primary uint64, fallback uint64) string {
	for _, value := range []uint64{primary, fallback} {
		if value == 0 {
			continue
		}
		return time.Unix(0, int64(value)).UTC().Format(time.RFC3339Nano)
	}
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func bodyToEventName(v *commonv1.AnyValue) string {
	value := anyValue(v)
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) != "" {
			return typed
		}
	case int64:
		return strconv.FormatInt(typed, 10)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	}
	return "otlp.log"
}

func stringValue(v any) string {
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func fallbackString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
