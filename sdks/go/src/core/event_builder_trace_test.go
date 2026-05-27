package core

import (
	"testing"
)

// TestBuildEventGeneratesTraceContext verifies that trace/span IDs are generated via ensureTraceContext.
// buildEvent no longer generates IDs (deferred to after sampling for performance).
// Requirements: 39.6
func TestBuildEventGeneratesTraceContext(t *testing.T) {
	cfg := Test()
	params := Params{
		Event: "test.event",
	}

	ev := buildEvent(params, &cfg)

	// buildEvent defers generation - IDs should be empty
	if ev.TraceID != "" {
		t.Errorf("expected empty trace_id before ensureTraceContext, got %s", ev.TraceID)
	}
	if ev.SpanID != "" {
		t.Errorf("expected empty span_id before ensureTraceContext, got %s", ev.SpanID)
	}

	// After ensureTraceContext, IDs should be generated
	ev.ensureTraceContext()

	if ev.TraceID == "" {
		t.Error("expected trace_id to be generated, got empty string")
	}
	if !IsValidTraceID(ev.TraceID) {
		t.Errorf("generated trace_id is not valid: %s", ev.TraceID)
	}
	if ev.SpanID == "" {
		t.Error("expected span_id to be generated, got empty string")
	}
	if !IsValidSpanID(ev.SpanID) {
		t.Errorf("generated span_id is not valid: %s", ev.SpanID)
	}
}

// TestBuildEventPreservesProvidedTraceContext verifies that buildEvent preserves provided trace context.
// Requirements: 39.3, 39.4, 39.5
func TestBuildEventPreservesProvidedTraceContext(t *testing.T) {
	cfg := Test()
	providedTraceID := "0af7651916cd43dd8448eb211c80319c"
	providedSpanID := "00f067aa0ba902b7"
	providedParentID := "00f067aa0ba902b6"
	
	params := Params{
		Event:    "test.event",
		TraceID:  providedTraceID,
		SpanID:   providedSpanID,
		ParentID: providedParentID,
	}
	
	ev := buildEvent(params, &cfg)
	
	// Verify trace_id is preserved
	if ev.TraceID != providedTraceID {
		t.Errorf("expected trace_id %s, got %s", providedTraceID, ev.TraceID)
	}
	
	// Verify span_id is preserved
	if ev.SpanID != providedSpanID {
		t.Errorf("expected span_id %s, got %s", providedSpanID, ev.SpanID)
	}
	
	// Verify parent_span_id is preserved
	if ev.ParentID != providedParentID {
		t.Errorf("expected parent_span_id %s, got %s", providedParentID, ev.ParentID)
	}
}

// TestBuildEventGeneratesTraceIDWhenOnlySpanIDProvided verifies that trace_id is generated
// via ensureTraceContext when only span_id is provided.
// Requirements: 39.6
func TestBuildEventGeneratesTraceIDWhenOnlySpanIDProvided(t *testing.T) {
	cfg := Test()
	providedSpanID := "00f067aa0ba902b7"

	params := Params{
		Event:  "test.event",
		SpanID: providedSpanID,
	}

	ev := buildEvent(params, &cfg)

	// buildEvent preserves provided span_id
	if ev.SpanID != providedSpanID {
		t.Errorf("expected span_id %s, got %s", providedSpanID, ev.SpanID)
	}

	// trace_id is deferred - should be empty before ensureTraceContext
	if ev.TraceID != "" {
		t.Errorf("expected empty trace_id before ensureTraceContext, got %s", ev.TraceID)
	}

	// After ensureTraceContext, trace_id should be generated, span_id preserved
	ev.ensureTraceContext()
	if ev.TraceID == "" {
		t.Error("expected trace_id to be generated, got empty string")
	}
	if !IsValidTraceID(ev.TraceID) {
		t.Errorf("generated trace_id is not valid: %s", ev.TraceID)
	}
	if ev.SpanID != providedSpanID {
		t.Errorf("expected span_id %s, got %s", providedSpanID, ev.SpanID)
	}
}

// TestBuildEventGeneratesSpanIDWhenOnlyTraceIDProvided verifies that span_id is generated
// when trace_id is provided. Trace/span generation is now deferred to ensureTraceContext().
// Requirements: 39.6
func TestBuildEventGeneratesSpanIDWhenOnlyTraceIDProvided(t *testing.T) {
	cfg := Test()
	providedTraceID := "0af7651916cd43dd8448eb211c80319c"

	params := Params{
		Event:   "test.event",
		TraceID: providedTraceID,
	}

	ev := buildEvent(params, &cfg)

	// Verify trace_id is preserved
	if ev.TraceID != providedTraceID {
		t.Errorf("expected trace_id %s, got %s", providedTraceID, ev.TraceID)
	}

	// span_id is deferred to ensureTraceContext, so it should be empty after buildEvent
	if ev.SpanID != "" {
		t.Errorf("expected empty span_id before ensureTraceContext, got %s", ev.SpanID)
	}

	// After ensureTraceContext, span_id should be generated
	ev.ensureTraceContext()
	if ev.SpanID == "" {
		t.Error("expected span_id to be generated, got empty string")
	}
	if !IsValidSpanID(ev.SpanID) {
		t.Errorf("generated span_id is not valid: %s", ev.SpanID)
	}
}

// TestBuildEventParentIDOptional verifies that parent_span_id is optional.
// Requirements: 39.5
func TestBuildEventParentIDOptional(t *testing.T) {
	cfg := Test()
	params := Params{
		Event: "test.event",
	}
	
	ev := buildEvent(params, &cfg)
	
	// Verify parent_span_id is empty when not provided
	if ev.ParentID != "" {
		t.Errorf("expected parent_span_id to be empty, got %s", ev.ParentID)
	}
}

// TestBuildEventTraceContextUniqueness verifies that each event gets unique trace context.
// Trace/span IDs are now generated lazily via ensureTraceContext() (after sampling),
// so buildEvent leaves them empty. This test verifies uniqueness after ensureTraceContext.
// Requirements: 39.6
func TestBuildEventTraceContextUniqueness(t *testing.T) {
	cfg := Test()

	ev1 := buildEvent(Params{Event: "test.event1"}, &cfg)
	ev2 := buildEvent(Params{Event: "test.event2"}, &cfg)

	// buildEvent no longer generates trace/span IDs (deferred to ensureTraceContext)
	if ev1.TraceID != "" || ev2.TraceID != "" {
		t.Errorf("expected empty trace IDs before ensureTraceContext, got %q and %q", ev1.TraceID, ev2.TraceID)
	}

	// After ensureTraceContext, IDs should be unique
	ev1.ensureTraceContext()
	ev2.ensureTraceContext()

	if ev1.TraceID == ev2.TraceID {
		t.Errorf("expected different trace IDs, got same: %s", ev1.TraceID)
	}
	if ev1.SpanID == ev2.SpanID {
		t.Errorf("expected different span IDs, got same: %s", ev1.SpanID)
	}
}
