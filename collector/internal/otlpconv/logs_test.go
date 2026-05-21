package otlpconv

import (
	"encoding/json"
	"testing"

	collectorlogsv1 "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	logsv1 "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcev1 "go.opentelemetry.io/proto/otlp/resource/v1"
)

func TestConvertExportLogsRequestBasic(t *testing.T) {
	req := &collectorlogsv1.ExportLogsServiceRequest{
		ResourceLogs: []*logsv1.ResourceLogs{
			{
				Resource: &resourcev1.Resource{Attributes: []*commonv1.KeyValue{{Key: "service.name", Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_StringValue{StringValue: "checkout"}}}}},
				ScopeLogs: []*logsv1.ScopeLogs{{LogRecords: []*logsv1.LogRecord{{TimeUnixNano: 1630000000000, Body: &commonv1.AnyValue{Value: &commonv1.AnyValue_StringValue{StringValue: "order.created"}}, SeverityText: "INFO"}}}},
			},
		},
	}
	out, err := ConvertExportLogsRequest(req, Config{SchemaVersion: "1", EventVersion: "1", DefaultService: "default"})
	if err != nil {
		t.Fatalf("convert error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 event, got %d", len(out))
	}
	var evt map[string]interface{}
	if err := json.Unmarshal(out[0], &evt); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if evt["service"] != "checkout" {
		t.Fatalf("expected service checkout, got %v", evt["service"])
	}
	if evt["event"] != "order.created" {
		t.Fatalf("expected event name, got %v", evt["event"])
	}
}
