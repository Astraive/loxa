package core

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

// TestTraceContextPropagation verifies that trace context is extracted from context.Context
// when StartEvent is called without explicit trace IDs in params.
// Requirements: 39.1, 39.2, 39.3, 39.4, 39.5, 39.6, 39.8
func TestTraceContextPropagation(t *testing.T) {
	t.Run("extracts trace context from OTel span context", func(t *testing.T) {
		// Create an OTel span context
		traceID := trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
		spanID := trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8}
		sc := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    traceID,
			SpanID:     spanID,
			TraceFlags: trace.FlagsSampled,
		})
		ctx := trace.ContextWithSpanContext(context.Background(), sc)

		// Create logger with OTelBridge enabled and start event without explicit trace IDs
		logger, err := New(Test().WithOTelBridge(true))
		if err != nil {
			t.Fatalf("failed to create logger: %v", err)
		}
		defer func() { _ = logger.Shutdown(context.Background()) }()

		ctx = logger.StartEvent(ctx, Params{Event: "test.event"})
		ev, ok := FromContext(ctx)
		if !ok {
			t.Fatal("expected event in context")
		}

		// Verify trace_id is extracted from OTel context
		expectedTraceID := traceID.String()
		if ev.TraceID != expectedTraceID {
			t.Errorf("expected trace_id=%q, got %q", expectedTraceID, ev.TraceID)
		}

		// Verify parent_span_id is set to the OTel span ID
		expectedParentSpanID := spanID.String()
		if ev.ParentID != expectedParentSpanID {
			t.Errorf("expected parent_span_id=%q, got %q", expectedParentSpanID, ev.ParentID)
		}

		// Verify span_id is deferred to ensureTraceContext (runs during Emit).
		// After StartEvent, span_id should be empty since it's generated lazily.
		if ev.SpanID != "" {
			t.Errorf("expected empty span_id before emit (deferred), got %q", ev.SpanID)
		}

		// After emit, span_id should be generated
		sink, store := MemorySink()
		logger2, _ := New(Test().WithSink(sink).WithOTelBridge(true))
		defer func() { _ = logger2.Shutdown(context.Background()) }()
		ctx2 := logger2.StartEvent(ctx, Params{Event: "test.event"})
		_ = logger2.Finish(ctx2, "success")
		_ = logger2.Emit(ctx2)
		_ = logger2.Flush(context.Background())
		events := store.Events()
		if len(events) == 1 && events[0].SpanID == expectedParentSpanID {
			t.Error("expected span_id to be different from parent_span_id")
		}
	})

	t.Run("generates new trace_id when none provided", func(t *testing.T) {
		// Create logger and start event without any trace context
		sink, store := MemorySink()
		logger, err := New(Test().WithSink(sink))
		if err != nil {
			t.Fatalf("failed to create logger: %v", err)
		}
		defer func() { _ = logger.Shutdown(context.Background()) }()

		ctx := logger.StartEvent(context.Background(), Params{Event: "test.event"})

		// Trace/span IDs are deferred to ensureTraceContext() which runs during Emit.
		// After StartEvent, IDs should be empty.
		ev, ok := FromContext(ctx)
		if !ok {
			t.Fatal("expected event in context")
		}
		if ev.TraceID != "" {
			t.Errorf("expected empty trace_id before emit, got %q", ev.TraceID)
		}
		if ev.SpanID != "" {
			t.Errorf("expected empty span_id before emit, got %q", ev.SpanID)
		}

		// Finish and emit - this triggers ensureTraceContext()
		if err := logger.Finish(ctx, "success"); err != nil {
			t.Fatalf("failed to finish: %v", err)
		}
		if err := logger.Emit(ctx); err != nil {
			t.Fatalf("failed to emit: %v", err)
		}
		if err := logger.Flush(context.Background()); err != nil {
			t.Fatalf("failed to flush: %v", err)
		}

		// Verify trace_id was generated during emit (Requirement 39.6)
		events := store.Events()
		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}
		emitted := events[0]
		if emitted.TraceID == "" {
			t.Error("expected trace_id to be generated")
		}
		if !IsValidTraceID(emitted.TraceID) {
			t.Errorf("expected valid trace_id, got %q", emitted.TraceID)
		}
		if emitted.SpanID == "" {
			t.Error("expected span_id to be generated")
		}
		if !IsValidSpanID(emitted.SpanID) {
			t.Errorf("expected valid span_id, got %q", emitted.SpanID)
		}
		if emitted.ParentID != "" {
			t.Errorf("expected empty parent_span_id, got %q", emitted.ParentID)
		}
	})

	t.Run("uses explicit trace IDs from params", func(t *testing.T) {
		// Create an OTel span context (should be ignored)
		traceID := trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
		spanID := trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8}
		sc := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    traceID,
			SpanID:     spanID,
			TraceFlags: trace.FlagsSampled,
		})
		ctx := trace.ContextWithSpanContext(context.Background(), sc)

		// Create logger and start event with explicit trace IDs
		logger, err := New(Test())
		if err != nil {
			t.Fatalf("failed to create logger: %v", err)
		}
		defer func() { _ = logger.Shutdown(context.Background()) }()

		explicitTraceID := "0af7651916cd43dd8448eb211c80319c"
		explicitSpanID := "00f067aa0ba902b7"
		explicitParentID := "b9c7c989f97918e1"

		ctx = logger.StartEvent(ctx, Params{
			Event:    "test.event",
			TraceID:  explicitTraceID,
			SpanID:   explicitSpanID,
			ParentID: explicitParentID,
		})
		ev, ok := FromContext(ctx)
		if !ok {
			t.Fatal("expected event in context")
		}

		// Verify explicit trace IDs are used (not extracted from context)
		if ev.TraceID != explicitTraceID {
			t.Errorf("expected trace_id=%q, got %q", explicitTraceID, ev.TraceID)
		}
		if ev.SpanID != explicitSpanID {
			t.Errorf("expected span_id=%q, got %q", explicitSpanID, ev.SpanID)
		}
		if ev.ParentID != explicitParentID {
			t.Errorf("expected parent_span_id=%q, got %q", explicitParentID, ev.ParentID)
		}
	})

	t.Run("includes trace context in emitted events", func(t *testing.T) {
		// Create memory sink to capture emitted events
		sink, store := MemorySink()
		logger, err := New(Test().WithSink(sink))
		if err != nil {
			t.Fatalf("failed to create logger: %v", err)
		}
		defer func() { _ = logger.Shutdown(context.Background()) }()

		// Create event with explicit trace context
		ctx := logger.StartEvent(context.Background(), Params{
			Event:   "test.event",
			TraceID: "0af7651916cd43dd8448eb211c80319c",
			SpanID:  "00f067aa0ba902b7",
		})

		// Finish and emit
		if err := logger.Finish(ctx, "success"); err != nil {
			t.Fatalf("failed to finish event: %v", err)
		}
		if err := logger.Emit(ctx); err != nil {
			t.Fatalf("failed to emit event: %v", err)
		}
		if err := logger.Flush(context.Background()); err != nil {
			t.Fatalf("failed to flush: %v", err)
		}

		// Verify trace context is preserved in emitted event (Requirement 39.9)
		events := store.Events()
		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}

		emittedEvent := events[0]
		if emittedEvent.TraceID != "0af7651916cd43dd8448eb211c80319c" {
			t.Errorf("expected trace_id=%q in emitted event, got %q", "0af7651916cd43dd8448eb211c80319c", emittedEvent.TraceID)
		}
		if emittedEvent.SpanID != "00f067aa0ba902b7" {
			t.Errorf("expected span_id=%q in emitted event, got %q", "00f067aa0ba902b7", emittedEvent.SpanID)
		}
	})
}

// TestGenerateTraceID verifies trace ID generation follows W3C spec.
// Requirements: 39.6
func TestGenerateTraceID(t *testing.T) {
	traceID := GenerateTraceID()
	
	// Verify length (32 hex characters = 16 bytes)
	if len(traceID) != 32 {
		t.Errorf("expected trace_id length 32, got %d", len(traceID))
	}
	
	// Verify it's valid hex
	if !IsValidTraceID(traceID) {
		t.Errorf("expected valid trace_id, got %q", traceID)
	}
	
	// Verify uniqueness (generate multiple and check they're different)
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := GenerateTraceID()
		if ids[id] {
			t.Errorf("generated duplicate trace_id: %q", id)
		}
		ids[id] = true
	}
}

// TestGenerateSpanID verifies span ID generation follows W3C spec.
// Requirements: 39.6
func TestGenerateSpanID(t *testing.T) {
	spanID := GenerateSpanID()
	
	// Verify length (16 hex characters = 8 bytes)
	if len(spanID) != 16 {
		t.Errorf("expected span_id length 16, got %d", len(spanID))
	}
	
	// Verify it's valid hex
	if !IsValidSpanID(spanID) {
		t.Errorf("expected valid span_id, got %q", spanID)
	}
	
	// Verify uniqueness (generate multiple and check they're different)
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := GenerateSpanID()
		if ids[id] {
			t.Errorf("generated duplicate span_id: %q", id)
		}
		ids[id] = true
	}
}

// TestIsValidTraceID verifies trace ID validation.
func TestIsValidTraceID(t *testing.T) {
	tests := []struct {
		name     string
		traceID  string
		expected bool
	}{
		{"valid trace ID", "0af7651916cd43dd8448eb211c80319c", true},
		{"valid trace ID uppercase", "0AF7651916CD43DD8448EB211C80319C", true},
		{"all zeros (invalid)", "00000000000000000000000000000000", false},
		{"too short", "0af7651916cd43dd", false},
		{"too long", "0af7651916cd43dd8448eb211c80319c00", false},
		{"invalid characters", "0af7651916cd43dd8448eb211c80319g", false},
		{"empty", "", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidTraceID(tt.traceID)
			if result != tt.expected {
				t.Errorf("IsValidTraceID(%q) = %v, expected %v", tt.traceID, result, tt.expected)
			}
		})
	}
}

// TestIsValidSpanID verifies span ID validation.
func TestIsValidSpanID(t *testing.T) {
	tests := []struct {
		name     string
		spanID   string
		expected bool
	}{
		{"valid span ID", "00f067aa0ba902b7", true},
		{"valid span ID uppercase", "00F067AA0BA902B7", true},
		{"all zeros (invalid)", "0000000000000000", false},
		{"too short", "00f067aa", false},
		{"too long", "00f067aa0ba902b700", false},
		{"invalid characters", "00f067aa0ba902bz", false},
		{"empty", "", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidSpanID(tt.spanID)
			if result != tt.expected {
				t.Errorf("IsValidSpanID(%q) = %v, expected %v", tt.spanID, result, tt.expected)
			}
		})
	}
}
