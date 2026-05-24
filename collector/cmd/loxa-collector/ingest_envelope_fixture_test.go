package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/astraive/loxa-collector/internal/ingest"
	speccontract "github.com/astraive/loxa/spec/generated/go/contract"
)

type ingestEnvelopeFixture struct {
	Name        string          `json:"name"`
	Mode        string          `json:"mode"`
	ContentType string          `json:"content_type"`
	Body        json.RawMessage `json:"body"`
	BodyLines   []string        `json:"body_lines"`
	Expected    struct {
		AcceptedEventCount int    `json:"accepted_event_count"`
		EventsCount        int    `json:"events_count"`
		FirstEventEvent    string `json:"first_event.event"`
		FirstEventService  string `json:"first_event.service"`
		APIVersion         string `json:"api_version"`
		SourceService      string `json:"source.service"`
	} `json:"expected"`
}

func firstFixtureMatch(patterns ...string) []string {
	for _, pattern := range patterns {
		files, _ := filepath.Glob(pattern)
		if len(files) > 0 {
			return files
		}
	}
	return nil
}

func TestIngestEnvelopeFixturesAreAcceptedByCollectorParser(t *testing.T) {
	files := firstFixtureMatch(
		filepath.Join("..", "..", "..", "spec", "fixtures", "ingest", "*.json"),
		filepath.Join("..", "..", "..", "spec", "examples", "golden", "ingest-envelopes", "*.json"),
	)
	if len(files) == 0 {
		t.Fatalf("no ingest fixtures found")
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var fixture ingestEnvelopeFixture
			if err := json.Unmarshal(raw, &fixture); err != nil {
				t.Fatalf("unmarshal fixture: %v", err)
			}

			body := fixture.Body
			if len(fixture.BodyLines) > 0 {
				body = []byte(bytes.Join(func() [][]byte {
					lines := make([][]byte, 0, len(fixture.BodyLines))
					for _, line := range fixture.BodyLines {
						lines = append(lines, []byte(line))
					}
					return lines
				}(), []byte("\n")))
			}

			req := httptest.NewRequest("POST", "/events", bytes.NewReader(body))
			req.Header.Set("Content-Type", fixture.ContentType)
			events, err := ingest.ParseEvents(req, 1<<20)
			if err != nil {
				t.Fatalf("parse events: %v", err)
			}

			expectedCount := fixture.Expected.AcceptedEventCount
			if expectedCount == 0 {
				expectedCount = fixture.Expected.EventsCount
			}
			if len(events) != expectedCount {
				t.Fatalf("expected %d events, got %d", expectedCount, len(events))
			}
			if expectedCount == 0 {
				return
			}

			var first map[string]any
			if err := json.Unmarshal(events[0], &first); err != nil {
				t.Fatalf("decode first event: %v", err)
			}
			if got := first["event"]; fixture.Expected.FirstEventEvent != "" && got != fixture.Expected.FirstEventEvent {
				t.Fatalf("expected first event %q, got %v", fixture.Expected.FirstEventEvent, got)
			}
			if got := first["service"]; fixture.Expected.FirstEventService != "" && got != fixture.Expected.FirstEventService {
				t.Fatalf("expected first service %q, got %v", fixture.Expected.FirstEventService, got)
			}

			if fixture.Mode == "wrapped_batch" {
				var envelope speccontract.IngestEnvelope
				if err := json.Unmarshal(fixture.Body, &envelope); err != nil {
					t.Fatalf("decode request envelope: %v", err)
				}
				if fixture.Expected.APIVersion != "" && envelope.APIVersion != fixture.Expected.APIVersion {
					t.Fatalf("expected api_version %q, got %q", fixture.Expected.APIVersion, envelope.APIVersion)
				}
				if fixture.Expected.SourceService != "" && envelope.Source.Service != fixture.Expected.SourceService {
					t.Fatalf("expected source.service %q, got %q", fixture.Expected.SourceService, envelope.Source.Service)
				}
			}
		})
	}
}
