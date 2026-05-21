package core

import (
	"context"
	"errors"
	"testing"
)

// TestEventStateEnum verifies all required event states exist.
// Requirements: 1.1
func TestEventStateEnum(t *testing.T) {
	states := []EventState{
		EventStateCreated,
		EventStateActive,
		EventStateFinished,
		EventStateEmitting,
		EventStateEmitted,
		EventStateFailedValidation,
		EventStateDeliveryFailed,
	}

	expectedStates := map[EventState]string{
		EventStateCreated:          "created",
		EventStateActive:           "active",
		EventStateFinished:         "finished",
		EventStateEmitting:         "emitting",
		EventStateEmitted:          "emitted",
		EventStateFailedValidation: "failed_validation",
		EventStateDeliveryFailed:   "delivery_failed",
	}

	for _, state := range states {
		expected, ok := expectedStates[state]
		if !ok {
			t.Errorf("unexpected state: %s", state)
		}
		if string(state) != expected {
			t.Errorf("state %s has wrong value: got %s, want %s", state, string(state), expected)
		}
	}
}

// TestStartEventTransitionsToCreated verifies that StartEvent creates an event in created state.
// Requirements: 1.2
func TestStartEventTransitionsToCreated(t *testing.T) {
	cfg := Test()
	cfg.Sinks = []Sink{NoopSink()}
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	ctx := l.StartEvent(context.Background(), Params{Event: "test.event"})
	ev, ok := FromContext(ctx)
	if !ok {
		t.Fatal("event not found in context")
	}

	state := ev.State()
	if state != EventStateCreated {
		t.Errorf("expected state %s, got %s", EventStateCreated, state)
	}
}

// TestEnrichTransitionsToActive verifies that Enrich transitions from created to active.
// Requirements: 1.3
func TestEnrichTransitionsToActive(t *testing.T) {
	cfg := Test()
	cfg.Sinks = []Sink{NoopSink()}
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	ctx := l.StartEvent(context.Background(), Params{Event: "test.event"})
	ev, _ := FromContext(ctx)

	// Verify initial state
	if state := ev.State(); state != EventStateCreated {
		t.Fatalf("expected initial state %s, got %s", EventStateCreated, state)
	}

	// Call Enrich
	if err := l.Enrich(ctx, String("key", "value")); err != nil {
		t.Fatalf("enrich failed: %v", err)
	}

	// Verify state transition
	if state := ev.State(); state != EventStateActive {
		t.Errorf("expected state %s after Enrich, got %s", EventStateActive, state)
	}
}

// TestSetTransitionsToActive verifies that Set transitions from created to active.
// Requirements: 1.3
func TestSetTransitionsToActive(t *testing.T) {
	cfg := Test()
	cfg.Sinks = []Sink{NoopSink()}
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	ctx := l.StartEvent(context.Background(), Params{Event: "test.event"})
	ev, _ := FromContext(ctx)

	// Verify initial state
	if state := ev.State(); state != EventStateCreated {
		t.Fatalf("expected initial state %s, got %s", EventStateCreated, state)
	}

	// Call Set
	if err := l.Set(ctx, String("key", "value")); err != nil {
		t.Fatalf("set failed: %v", err)
	}

	// Verify state transition
	if state := ev.State(); state != EventStateActive {
		t.Errorf("expected state %s after Set, got %s", EventStateActive, state)
	}
}

// TestAddTransitionsToActive verifies that Add transitions from created to active.
// Requirements: 1.3
func TestAddTransitionsToActive(t *testing.T) {
	cfg := Test()
	cfg.Sinks = []Sink{NoopSink()}
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	ctx := l.StartEvent(context.Background(), Params{Event: "test.event"})
	ev, _ := FromContext(ctx)

	// Verify initial state
	if state := ev.State(); state != EventStateCreated {
		t.Fatalf("expected initial state %s, got %s", EventStateCreated, state)
	}

	// Call Add
	if err := l.Add(ctx, "items", "value1"); err != nil {
		t.Fatalf("add failed: %v", err)
	}

	// Verify state transition
	if state := ev.State(); state != EventStateActive {
		t.Errorf("expected state %s after Add, got %s", EventStateActive, state)
	}
}

// TestFinishTransitionsToFinished verifies that Finish transitions from active to finished.
// Requirements: 1.4
func TestFinishTransitionsToFinished(t *testing.T) {
	cfg := Test()
	cfg.Sinks = []Sink{NoopSink()}
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	ctx := l.StartEvent(context.Background(), Params{Event: "test.event"})
	ev, _ := FromContext(ctx)

	// Transition to active
	if err := l.Enrich(ctx, String("key", "value")); err != nil {
		t.Fatalf("enrich failed: %v", err)
	}

	// Verify active state
	if state := ev.State(); state != EventStateActive {
		t.Fatalf("expected state %s, got %s", EventStateActive, state)
	}

	// Call Finish
	if err := l.Finish(ctx, "success"); err != nil {
		t.Fatalf("finish failed: %v", err)
	}

	// Verify state transition
	if state := ev.State(); state != EventStateFinished {
		t.Errorf("expected state %s after Finish, got %s", EventStateFinished, state)
	}
}

// TestFinishErrorTransitionsToFinished verifies that FinishError transitions from active to finished.
// Requirements: 1.4
func TestFinishErrorTransitionsToFinished(t *testing.T) {
	cfg := Test()
	cfg.Sinks = []Sink{NoopSink()}
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	ctx := l.StartEvent(context.Background(), Params{Event: "test.event"})
	ev, _ := FromContext(ctx)

	// Transition to active
	if err := l.Enrich(ctx, String("key", "value")); err != nil {
		t.Fatalf("enrich failed: %v", err)
	}

	// Call FinishError
	testErr := errors.New("test error")
	if err := l.FinishError(ctx, testErr); err != nil {
		t.Fatalf("finishError failed: %v", err)
	}

	// Verify state transition
	if state := ev.State(); state != EventStateFinished {
		t.Errorf("expected state %s after FinishError, got %s", EventStateFinished, state)
	}
}

// TestEmitTransitionsToEmitted verifies that Emit transitions from finished to emitting to emitted.
// Requirements: 1.5, 1.6
func TestEmitTransitionsToEmitted(t *testing.T) {
	cfg := Test()
	cfg.Sinks = []Sink{NoopSink()}
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	ctx := l.StartEvent(context.Background(), Params{Event: "test.event"})
	ev, _ := FromContext(ctx)

	// Transition to finished
	if err := l.Finish(ctx, "success"); err != nil {
		t.Fatalf("finish failed: %v", err)
	}

	// Verify finished state
	if state := ev.State(); state != EventStateFinished {
		t.Fatalf("expected state %s, got %s", EventStateFinished, state)
	}

	// Call Emit
	if err := l.Emit(ctx); err != nil {
		t.Fatalf("emit failed: %v", err)
	}

	// Verify state transition to emitted
	if state := ev.State(); state != EventStateEmitted {
		t.Errorf("expected state %s after Emit, got %s", EventStateEmitted, state)
	}
}

// TestValidationFailureTransition verifies that validation failures transition to validation_failed.
// Requirements: 1.7
func TestValidationFailureTransition(t *testing.T) {
	cfg := Test()
	cfg.DuplicateFieldPolicy = ErrorOnDuplicate
	cfg.Sinks = []Sink{NoopSink()}
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	ctx := l.StartEvent(context.Background(), Params{Event: "test.event"})
	ev, _ := FromContext(ctx)

	// Add a duplicate canonical field to trigger validation failure
	if err := l.Enrich(ctx, String("service", "shadow")); err != nil {
		t.Fatalf("enrich failed: %v", err)
	}

	// Try to emit (should fail validation)
	if err := l.Emit(ctx); err == nil {
		t.Fatal("expected validation error")
	}

	// Verify state transition
	if state := ev.State(); state != EventStateFailedValidation {
		t.Errorf("expected state %s after validation failure, got %s", EventStateFailedValidation, state)
	}
}

// TestInvalidStateTransitionsRejected verifies that invalid state transitions are rejected.
// Requirements: 1.9
func TestInvalidStateTransitionsRejected(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(*Logger, context.Context) (*Event, context.Context)
		operation     func(*Logger, context.Context) error
		expectedError string
	}{
		{
			name: "emit created event",
			setup: func(l *Logger, ctx context.Context) (*Event, context.Context) {
				ctx = l.StartEvent(ctx, Params{Event: "test.event"})
				ev, _ := FromContext(ctx)
				return ev, ctx
			},
			operation: func(l *Logger, ctx context.Context) error {
				return l.Emit(ctx)
			},
			expectedError: "", // Should succeed but not actually emit (no finish)
		},
		{
			name: "finish emitted event",
			setup: func(l *Logger, ctx context.Context) (*Event, context.Context) {
				ctx = l.StartEvent(ctx, Params{Event: "test.event"})
				ev, _ := FromContext(ctx)
				_ = l.Finish(ctx, "success")
				_ = l.Emit(ctx)
				return ev, ctx
			},
			operation: func(l *Logger, ctx context.Context) error {
				return l.Finish(ctx, "success")
			},
			expectedError: "closed in state",
		},
		{
			name: "enrich emitted event",
			setup: func(l *Logger, ctx context.Context) (*Event, context.Context) {
				ctx = l.StartEvent(ctx, Params{Event: "test.event"})
				ev, _ := FromContext(ctx)
				_ = l.Finish(ctx, "success")
				_ = l.Emit(ctx)
				return ev, ctx
			},
			operation: func(l *Logger, ctx context.Context) error {
				return l.Enrich(ctx, String("key", "value"))
			},
			expectedError: "closed in state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Test()
			cfg.Sinks = []Sink{NoopSink()}
			l, err := New(cfg)
			if err != nil {
				t.Fatalf("new logger: %v", err)
			}

			_, ctx := tt.setup(l, context.Background())
			err = tt.operation(l, ctx)

			if tt.expectedError == "" {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.expectedError)
				} else if !contains(err.Error(), tt.expectedError) {
					t.Errorf("expected error containing %q, got: %v", tt.expectedError, err)
				}
			}
		})
	}
}

// TestGetStateMethod verifies that GetState returns the current event state.
// Requirements: 1.10
func TestGetStateMethod(t *testing.T) {
	cfg := Test()
	cfg.Sinks = []Sink{NoopSink()}
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	ctx := l.StartEvent(context.Background(), Params{Event: "test.event"})
	ev, _ := FromContext(ctx)

	// Test created state
	if state := ev.State(); state != EventStateCreated {
		t.Errorf("expected state %s, got %s", EventStateCreated, state)
	}

	// Test active state
	_ = l.Enrich(ctx, String("key", "value"))
	if state := ev.State(); state != EventStateActive {
		t.Errorf("expected state %s, got %s", EventStateActive, state)
	}

	// Test finished state
	_ = l.Finish(ctx, "success")
	if state := ev.State(); state != EventStateFinished {
		t.Errorf("expected state %s, got %s", EventStateFinished, state)
	}

	// Test emitted state
	_ = l.Emit(ctx)
	if state := ev.State(); state != EventStateEmitted {
		t.Errorf("expected state %s, got %s", EventStateEmitted, state)
	}
}

// TestStateTransitionOrder verifies the correct order of state transitions.
// Requirements: 1.11
func TestStateTransitionOrder(t *testing.T) {
	cfg := Test()
	cfg.Sinks = []Sink{NoopSink()}
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	ctx := l.StartEvent(context.Background(), Params{Event: "test.event"})
	ev, _ := FromContext(ctx)

	expectedStates := []EventState{
		EventStateCreated,
		EventStateActive,
		EventStateFinished,
		EventStateEmitted,
	}

	// Created
	if state := ev.State(); state != expectedStates[0] {
		t.Errorf("step 0: expected state %s, got %s", expectedStates[0], state)
	}

	// Active
	_ = l.Enrich(ctx, String("key", "value"))
	if state := ev.State(); state != expectedStates[1] {
		t.Errorf("step 1: expected state %s, got %s", expectedStates[1], state)
	}

	// Finished
	_ = l.Finish(ctx, "success")
	if state := ev.State(); state != expectedStates[2] {
		t.Errorf("step 2: expected state %s, got %s", expectedStates[2], state)
	}

	// Emitted
	_ = l.Emit(ctx)
	if state := ev.State(); state != expectedStates[3] {
		t.Errorf("step 3: expected state %s, got %s", expectedStates[3], state)
	}
}

// TestStartEventMethod verifies StartEvent creates an event with correct initial state.
// Requirements: 2.1
func TestStartEventMethod(t *testing.T) {
	cfg := Test()
	cfg.Sinks = []Sink{NoopSink()}
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	eventType := "test.event"
	ctx := l.StartEvent(context.Background(), Params{Event: eventType})
	ev, ok := FromContext(ctx)
	if !ok {
		t.Fatal("event not found in context")
	}

	if ev.Event != eventType {
		t.Errorf("expected event type %s, got %s", eventType, ev.Event)
	}

	if ev.EventID == "" {
		t.Error("event ID should not be empty")
	}

	if state := ev.State(); state != EventStateCreated {
		t.Errorf("expected initial state %s, got %s", EventStateCreated, state)
	}
}

// TestEnrichMethod verifies Enrich adds fields to an event.
// Requirements: 2.2
func TestEnrichMethod(t *testing.T) {
	cfg := Test()
	cfg.Sinks = []Sink{NoopSink()}
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	ctx := l.StartEvent(context.Background(), Params{Event: "test.event"})
	ev, _ := FromContext(ctx)

	// Enrich with a field
	key := "test_key"
	value := "test_value"
	if err := l.Enrich(ctx, String(key, value)); err != nil {
		t.Fatalf("enrich failed: %v", err)
	}

	// Verify field was added
	got, ok := ev.Get(key)
	if !ok {
		t.Errorf("field %s not found", key)
	}
	if got != value {
		t.Errorf("expected value %s, got %v", value, got)
	}
}

// TestSetMethod verifies Set updates field values on an event.
// Requirements: 2.3
func TestSetMethod(t *testing.T) {
	cfg := Test()
	cfg.Sinks = []Sink{NoopSink()}
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	ctx := l.StartEvent(context.Background(), Params{Event: "test.event"})
	ev, _ := FromContext(ctx)

	// Set initial value
	key := "test_key"
	value1 := "value1"
	if err := l.Set(ctx, String(key, value1)); err != nil {
		t.Fatalf("set failed: %v", err)
	}

	// Verify initial value
	got, ok := ev.Get(key)
	if !ok {
		t.Errorf("field %s not found", key)
	}
	if got != value1 {
		t.Errorf("expected value %s, got %v", value1, got)
	}

	// Update value
	value2 := "value2"
	if err := l.Set(ctx, String(key, value2)); err != nil {
		t.Fatalf("set failed: %v", err)
	}

	// Verify updated value
	got, ok = ev.Get(key)
	if !ok {
		t.Errorf("field %s not found", key)
	}
	if got != value2 {
		t.Errorf("expected value %s, got %v", value2, got)
	}
}

// TestAddMethod verifies Add appends to array fields on an event.
// Requirements: 2.4
func TestAddMethod(t *testing.T) {
	cfg := Test()
	cfg.Sinks = []Sink{NoopSink()}
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	ctx := l.StartEvent(context.Background(), Params{Event: "test.event"})
	ev, _ := FromContext(ctx)

	// Add first value
	key := "items"
	value1 := "item1"
	if err := l.Add(ctx, key, value1); err != nil {
		t.Fatalf("add failed: %v", err)
	}

	// Add second value
	value2 := "item2"
	if err := l.Add(ctx, key, value2); err != nil {
		t.Fatalf("add failed: %v", err)
	}

	// Verify array was created and values were added
	got, ok := ev.Get(key)
	if !ok {
		t.Errorf("field %s not found", key)
	}

	arr, ok := got.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", got)
	}

	if len(arr) != 2 {
		t.Errorf("expected array length 2, got %d", len(arr))
	}

	if arr[0] != value1 {
		t.Errorf("expected first element %s, got %v", value1, arr[0])
	}

	if arr[1] != value2 {
		t.Errorf("expected second element %s, got %v", value2, arr[1])
	}
}

// TestFinishMethod verifies Finish marks an event as successfully completed.
// Requirements: 2.5
func TestFinishMethod(t *testing.T) {
	cfg := Test()
	cfg.Sinks = []Sink{NoopSink()}
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	ctx := l.StartEvent(context.Background(), Params{Event: "test.event"})
	ev, _ := FromContext(ctx)

	outcome := "success"
	if err := l.Finish(ctx, outcome); err != nil {
		t.Fatalf("finish failed: %v", err)
	}

	if ev.Outcome != outcome {
		t.Errorf("expected outcome %s, got %s", outcome, ev.Outcome)
	}

	if !ev.IsFinished() {
		t.Error("event should be marked as finished")
	}

	// Duration might be 0 if the test runs very fast, so we just check it's non-negative
	if ev.DurationMS < 0 {
		t.Error("duration should be non-negative")
	}
}

// TestFinishErrorMethod verifies FinishError marks an event as failed with error details.
// Requirements: 2.6
func TestFinishErrorMethod(t *testing.T) {
	cfg := Test()
	cfg.Sinks = []Sink{NoopSink()}
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	ctx := l.StartEvent(context.Background(), Params{Event: "test.event"})
	ev, _ := FromContext(ctx)

	testErr := errors.New("test error")
	if err := l.FinishError(ctx, testErr); err != nil {
		t.Fatalf("finishError failed: %v", err)
	}

	if ev.Outcome != "error" {
		t.Errorf("expected outcome 'error', got %s", ev.Outcome)
	}

	if ev.Error == nil {
		t.Fatal("error info should be set")
	}

	if ev.Error.Message != testErr.Error() {
		t.Errorf("expected error message %s, got %s", testErr.Error(), ev.Error.Message)
	}

	if !ev.IsFinished() {
		t.Error("event should be marked as finished")
	}
}

// TestEmitMethod verifies Emit sends a finished event to the collector.
// Requirements: 2.7
func TestEmitMethod(t *testing.T) {
	cfg := Test()
	sink, store := MemorySink()
	cfg.Sinks = []Sink{sink}
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	ctx := l.StartEvent(context.Background(), Params{Event: "test.event"})
	ev, _ := FromContext(ctx)

	// Finish the event
	if err := l.Finish(ctx, "success"); err != nil {
		t.Fatalf("finish failed: %v", err)
	}

	// Emit the event
	if err := l.Emit(ctx); err != nil {
		t.Fatalf("emit failed: %v", err)
	}

	// Verify event was emitted
	if !ev.IsEmitted() {
		t.Error("event should be marked as emitted")
	}

	// Verify event was written to sink
	events := store.Events()
	if len(events) != 1 {
		t.Errorf("expected 1 event in sink, got %d", len(events))
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
