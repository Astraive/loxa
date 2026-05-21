package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type collectorAckFixture struct {
	HTTPStatus int             `json:"http_status"`
	Response   json.RawMessage `json:"response"`
	Expected   struct {
		Outcome         string `json:"outcome"`
		MessageContains string `json:"message_contains"`
	} `json:"expected"`
}

func firstMatch(patterns ...string) []string {
	for _, pattern := range patterns {
		files, _ := filepath.Glob(pattern)
		if len(files) > 0 {
			return files
		}
	}
	return nil
}

func TestCollectorAckBehaviorFixtures(t *testing.T) {
	files := firstMatch(
		filepath.Join("..", "..", "..", "..", "spec", "fixtures", "collector-responses", "*.json"),
		filepath.Join("..", "..", "..", "..", "spec", "examples", "golden", "collector-acks", "*.json"),
	)
	if len(files) == 0 {
		t.Fatalf("no collector ack fixtures found")
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var fixture collectorAckFixture
			if err := json.Unmarshal(raw, &fixture); err != nil {
				t.Fatalf("unmarshal fixture: %v", err)
			}

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(fixture.HTTPStatus)
				_, _ = w.Write(fixture.Response)
			}))
			defer srv.Close()

			sink, err := CollectorSink(CollectorSinkConfig{
				Endpoint:          srv.URL,
				MaxRetries:        0,
				MaxBackoff:        10 * time.Millisecond,
				Timeout:           2 * time.Second,
				ConnectionTimeout: 2 * time.Second,
				Service:           "checkout",
			})
			if err != nil {
				t.Fatalf("collector sink: %v", err)
			}

			ev := &Event{Service: "checkout", Event: "payment.completed"}
			err = sink.WriteEvent(context.Background(), []byte(`{"event_id":"evt_1","service":"checkout","event":"payment.completed"}`), ev)
			switch fixture.Expected.Outcome {
			case "success":
				if err != nil {
					t.Fatalf("expected success, got %v", err)
				}
			case "failure":
				if err == nil {
					t.Fatalf("expected failure")
				}
				if fixture.Expected.MessageContains != "" && !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(fixture.Expected.MessageContains)) {
					t.Fatalf("expected error containing %q, got %v", fixture.Expected.MessageContains, err)
				}
			default:
				t.Fatalf("unknown expected outcome %q", fixture.Expected.Outcome)
			}
		})
	}
}
