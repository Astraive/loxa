package core

import "testing"

func TestEventCloneDeepCopy(t *testing.T) {
	ev := &Event{
		EventID: "evt-1",
		Attrs: []Attr{
			Group("user", String("id", "u-1")),
		},
		Checkpoints: []EventCheckpoint{
			{Name: "started", AtMS: 5, Attrs: []Attr{String("stage", "one")}},
		},
		Error: &ErrorInfo{Type: "x", Message: "boom"},
	}
	_ = ev.MarkEmitted()

	clone := ev.Clone()
	if clone == nil {
		t.Fatalf("expected clone")
	}
	if clone == ev {
		t.Fatalf("expected distinct clone pointer")
	}
	if clone.IsEmitted() {
		t.Fatalf("expected clone emitted flag to be reset")
	}
	if clone.Error == ev.Error {
		t.Fatalf("expected cloned error pointer")
	}

	clone.Error.Message = "changed"
	if ev.Error.Message != "boom" {
		t.Fatalf("expected original error to remain unchanged")
	}

	groupAttrs, ok := clone.Attrs[0].Value.([]Attr)
	if !ok {
		t.Fatalf("expected grouped attrs in clone")
	}
	groupAttrs[0].Value = "u-2"
	clone.Attrs[0].Value = groupAttrs

	origGroup, ok := ev.Attrs[0].Value.([]Attr)
	if !ok {
		t.Fatalf("expected grouped attrs in original")
	}
	if origGroup[0].Value.(string) != "u-1" {
		t.Fatalf("expected original attr to remain unchanged")
	}
}

