package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type ingestEnvelopeFixture struct {
	InputEvents []map[string]any `json:"input_events"`
	Expected    struct {
		APIVersion      string `json:"api_version"`
		SourceService   string `json:"source.service"`
		EventsCount     int    `json:"events_count"`
		FirstEventEvent string `json:"first_event.event"`
		FirstEventSvc   string `json:"first_event.service"`
	} `json:"expected"`
}

func firstExistingPath(paths ...string) string {
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return paths[0]
}

func TestCollectorWrappedBatchEnvelopeFixture(t *testing.T) {
	path := firstExistingPath(
		filepath.Join("..", "..", "..", "loxa-spec", "fixtures", "ingest", "wrapped_batch_json.json"),
		filepath.Join("..", "..", "..", "loxa-spec", "examples", "golden", "ingest-envelopes", "wrapped_batch_json.json"),
	)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fixture ingestEnvelopeFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	if len(fixture.InputEvents) != 1 {
		t.Fatalf("expected exactly one input event")
	}
	encoded, err := json.Marshal(fixture.InputEvents[0])
	if err != nil {
		t.Fatalf("marshal input event: %v", err)
	}
	sink := &collectorSink{cfg: CollectorSinkConfig{
		SDKName:    "loxa-go",
		SDKVersion: "1.0.0",
		Service:    "checkout",
	}}
	envelope, ok := sink.envelope(encoded, &Event{Service: "checkout"})
	if !ok {
		t.Fatalf("expected canonical collector envelope")
	}
	if err := ValidateIngestEnvelopeBytes(envelope, false); err != nil {
		t.Fatalf("expected envelope validation to pass: %v", err)
	}

	var payload struct {
		APIVersion string `json:"api_version"`
		Source     struct {
			Service string `json:"service"`
		} `json:"source"`
		Events []map[string]any `json:"events"`
	}
	if err := json.Unmarshal(envelope, &payload); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if payload.APIVersion != fixture.Expected.APIVersion {
		t.Fatalf("expected api_version %q, got %q", fixture.Expected.APIVersion, payload.APIVersion)
	}
	if payload.Source.Service != fixture.Expected.SourceService {
		t.Fatalf("expected source.service %q, got %q", fixture.Expected.SourceService, payload.Source.Service)
	}
	if len(payload.Events) != fixture.Expected.EventsCount {
		t.Fatalf("expected %d events, got %d", fixture.Expected.EventsCount, len(payload.Events))
	}
	if got := payload.Events[0]["event"]; got != fixture.Expected.FirstEventEvent {
		t.Fatalf("expected first event %q, got %v", fixture.Expected.FirstEventEvent, got)
	}
	if got := payload.Events[0]["service"]; got != fixture.Expected.FirstEventSvc {
		t.Fatalf("expected first service %q, got %v", fixture.Expected.FirstEventSvc, got)
	}
}

func TestValidateIngestEnvelopeBytesRejectsMissingEvents(t *testing.T) {
	raw := []byte(`{"api_version":"v1","source":{"sdk":"loxa-go","version":"1.0.0","service":"checkout"}}`)
	if err := ValidateIngestEnvelopeBytes(raw, false); err == nil {
		t.Fatal("expected missing events envelope to fail validation")
	}
}
