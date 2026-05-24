package core

import (
	"sync"
	"testing"
	"time"
)

func TestProcess_BasicFlow(t *testing.T) {
	ev := newTestEvent(t)
	proc, err := ev.StartProcess("redirect_to_gateway")
	if err != nil {
		t.Fatalf("StartProcess: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	_ = proc.Finish(Int("status_code", 302), String("gateway", "stripe"))

	if len(ev.Processes) != 1 {
		t.Fatalf("expected 1 process, got %d", len(ev.Processes))
	}
	p := ev.Processes[0]
	if p.Step != 1 {
		t.Errorf("expected step 1, got %d", p.Step)
	}
	if p.Name != "redirect_to_gateway" {
		t.Errorf("expected name redirect_to_gateway, got %s", p.Name)
	}
	if p.DurationMS < 1 {
		t.Errorf("expected duration >= 1ms, got %d", p.DurationMS)
	}
	if p.StartedAtMS < 0 {
		t.Errorf("expected started_at_ms >= 0, got %d", p.StartedAtMS)
	}
	if p.EndedAtMS <= p.StartedAtMS {
		t.Errorf("expected ended_at_ms > started_at_ms")
	}
}

func TestProcess_FinishError(t *testing.T) {
	ev := newTestEvent(t)
	proc, _ := ev.StartProcess("payment_attempt")
	time.Sleep(2 * time.Millisecond)
	_ = proc.FinishError(errTest, 504, String("error_code", "gateway_timeout"))

	if len(ev.Processes) != 1 {
		t.Fatalf("expected 1 process, got %d", len(ev.Processes))
	}
	p := ev.Processes[0]
	if p.StatusCode != 504 {
		t.Errorf("expected status_code 504, got %d", p.StatusCode)
	}
}

func TestProcess_StepCounter(t *testing.T) {
	ev := newTestEvent(t)
	for i := 1; i <= 3; i++ {
		proc, _ := ev.StartProcess("step")
		_ = proc.Finish()
	}
	if len(ev.Processes) != 3 {
		t.Fatalf("expected 3 processes, got %d", len(ev.Processes))
	}
	for i, p := range ev.Processes {
		if p.Step != i+1 {
			t.Errorf("process %d: expected step %d, got %d", i, i+1, p.Step)
		}
	}
}

func TestTimer_StartStop(t *testing.T) {
	ev := newTestEvent(t)
	timer, err := ev.StartTimer("stripe.create_session")
	if err != nil {
		t.Fatalf("StartTimer: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	_ = timer.Stop(Int("status_code", 200))

	if len(ev.Timers) != 1 {
		t.Fatalf("expected 1 timer, got %d", len(ev.Timers))
	}
	tr := ev.Timers[0]
	if tr.Name != "stripe.create_session" {
		t.Errorf("expected name stripe.create_session, got %s", tr.Name)
	}
	if tr.DurationMS < 1 {
		t.Errorf("expected duration >= 1ms, got %d", tr.DurationMS)
	}
	if tr.StatusCode != 200 {
		t.Errorf("expected status_code 200, got %d", tr.StatusCode)
	}
}

func TestGroup_StartFinish(t *testing.T) {
	ev := newTestEvent(t)
	group, err := ev.StartGroup("payment_flow")
	if err != nil {
		t.Fatalf("StartGroup: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	_ = group.Finish(Int("status_code", 402), String("final_reason", "insufficient_funds"))

	if len(ev.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(ev.Groups))
	}
	g := ev.Groups[0]
	if g.Name != "payment_flow" {
		t.Errorf("expected name payment_flow, got %s", g.Name)
	}
	if g.DurationMS < 1 {
		t.Errorf("expected duration >= 1ms, got %d", g.DurationMS)
	}
	if g.StatusCode != 402 {
		t.Errorf("expected status_code 402, got %d", g.StatusCode)
	}
}

func TestStopwatch_Elapsed(t *testing.T) {
	sw := Stopwatch()
	time.Sleep(10 * time.Millisecond)
	elapsed := sw.Elapsed()
	if elapsed < 10*time.Millisecond {
		t.Errorf("expected elapsed >= 10ms, got %v", elapsed)
	}
}

func TestTiming_Concurrent(t *testing.T) {
	ev := newTestEvent(t)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			proc, _ := ev.StartProcess("concurrent_step")
			time.Sleep(time.Millisecond)
			_ = proc.Finish()
		}()
	}
	wg.Wait()
	if len(ev.Processes) != 100 {
		t.Errorf("expected 100 processes, got %d", len(ev.Processes))
	}
}

func TestTiming_WireFormat(t *testing.T) {
	ev := newTestEvent(t)
	ev.StartedAt = time.Now()

	proc, _ := ev.StartProcess("redirect_to_gateway")
	time.Sleep(5 * time.Millisecond)
	_ = proc.Finish(Int("status_code", 302), String("gateway", "stripe"))

	group, _ := ev.StartGroup("payment_flow")
	time.Sleep(5 * time.Millisecond)
	_ = group.Finish(Int("status_code", 402))

	timer, _ := ev.StartTimer("stripe.create_session")
	time.Sleep(5 * time.Millisecond)
	_ = timer.Stop(Int("status_code", 200))

	// Verify process
	if len(ev.Processes) != 1 {
		t.Fatalf("expected 1 process, got %d", len(ev.Processes))
	}
	p := ev.Processes[0]
	if p.Step != 1 {
		t.Errorf("expected step 1, got %d", p.Step)
	}
	if p.Name != "redirect_to_gateway" {
		t.Errorf("expected name redirect_to_gateway, got %s", p.Name)
	}
	if p.StatusCode != 302 {
		t.Errorf("expected status_code 302, got %d", p.StatusCode)
	}
	if p.DurationMS < 1 {
		t.Errorf("expected duration_ms >= 1, got %d", p.DurationMS)
	}
	if p.StartedAtMS < 0 {
		t.Errorf("expected started_at_ms >= 0, got %d", p.StartedAtMS)
	}
	if p.EndedAtMS <= p.StartedAtMS {
		t.Errorf("expected ended_at_ms > started_at_ms")
	}

	// Verify groups
	if len(ev.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(ev.Groups))
	}
	g := ev.Groups[0]
	if g.Name != "payment_flow" {
		t.Errorf("expected name payment_flow, got %s", g.Name)
	}
	if g.StatusCode != 402 {
		t.Errorf("expected status_code 402, got %d", g.StatusCode)
	}
	if g.DurationMS < 1 {
		t.Errorf("expected duration_ms >= 1, got %d", g.DurationMS)
	}

	// Verify timers
	if len(ev.Timers) != 1 {
		t.Fatalf("expected 1 timer, got %d", len(ev.Timers))
	}
	tr := ev.Timers[0]
	if tr.Name != "stripe.create_session" {
		t.Errorf("expected name stripe.create_session, got %s", tr.Name)
	}
	if tr.StatusCode != 200 {
		t.Errorf("expected status_code 200, got %d", tr.StatusCode)
	}
	if tr.DurationMS < 1 {
		t.Errorf("expected duration_ms >= 1, got %d", tr.DurationMS)
	}
}

var errTest = &testError{msg: "test error"}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

func newTestEvent(t *testing.T) *Event {
	t.Helper()
	ev := &Event{
		EventID:    "evt_test",
		Timestamp:  time.Now(),
		StartedAt:  time.Now(),
		Event:      "test.event",
		Kind:       "event",
		Level:      LevelInfo,
		state:      EventStateActive,
	}
	return ev
}
