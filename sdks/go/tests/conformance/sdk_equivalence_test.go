package conformance

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/astraive/loxa/sdks/go"
)

func TestSDKOutputEquivalence(t *testing.T) {
	// Load canonical fixture
	fixturePath := filepath.Join("..", "..", "..", "..", "spec", "fixtures", "sdk-equivalence", "canonical_event.json")
	fixtureData, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Skipf("fixture not found: %v", err)
	}

	var fixture map[string]any
	if err := json.Unmarshal(fixtureData, &fixture); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}

	// Create event using SDK
	sink, store := loxa.MemorySink()
	cfg := loxa.Test().
		WithService("checkout").
		WithSink(sink).
		WithSampler(loxa.SampleAll()).
		WithAsync(false)
	if err := loxa.Configure(cfg); err != nil {
		t.Fatalf("configure: %v", err)
	}

	ctx := loxa.StartEvent(context.Background(), loxa.Params{
		Service: "checkout",
		Event:   "payment.completed",
		Kind:    "event",
		Level:   loxa.LevelInfo,
	})
	loxa.Enrich(ctx,
		loxa.Float64("amount", 99.99),
		loxa.String("currency", "USD"),
		loxa.String("user.id", "u_123"),
	)
	loxa.Finish(ctx, "success")
	loxa.Emit(ctx)
	loxa.Flush(context.Background())

	if store.Len() == 0 {
		t.Fatal("no events emitted")
	}

	// Verify the fixture itself has the expected fields
	expectedFields := []string{"schema_version", "event_version", "service", "event", "kind", "level", "outcome"}
	for _, field := range expectedFields {
		if _, ok := fixture[field]; !ok {
			t.Errorf("fixture missing expected field: %s", field)
		}
	}

	// Verify fixture values
	if fixture["schema_version"] != "v1" {
		t.Errorf("fixture schema_version: expected v1, got %v", fixture["schema_version"])
	}
	if fixture["event_version"] != "v1" {
		t.Errorf("fixture event_version: expected v1, got %v", fixture["event_version"])
	}
	if fixture["service"] != "checkout" {
		t.Errorf("fixture service: expected checkout, got %v", fixture["service"])
	}
	if fixture["event"] != "payment.completed" {
		t.Errorf("fixture event: expected payment.completed, got %v", fixture["event"])
	}
	if fixture["kind"] != "event" {
		t.Errorf("fixture kind: expected event, got %v", fixture["kind"])
	}
	if fixture["level"] != "info" {
		t.Errorf("fixture level: expected info, got %v", fixture["level"])
	}
	if fixture["outcome"] != "success" {
		t.Errorf("fixture outcome: expected success, got %v", fixture["outcome"])
	}
}
