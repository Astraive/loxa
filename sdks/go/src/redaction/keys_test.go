package redaction

import (
	"testing"

	"github.com/astraive/loza/sdks/go/src/core"
)

func TestRedactionConstructorsApplyExpectedPolicies(t *testing.T) {
	if _, keep := RedactKeys("password").Redact("name", "alice"); !keep {
		t.Fatal("RedactKeys unmatched field was dropped")
	}
	if value, keep := HashKeys("token").Redact("token", "abc"); !keep || value == "abc" {
		t.Fatalf("HashKeys result = (%v, %v), want changed and kept", value, keep)
	}
	if value, keep := MaskKeys("token").Redact("token", "abcdef"); !keep || value == "abcdef" {
		t.Fatalf("MaskKeys result = (%v, %v), want changed and kept", value, keep)
	}
	if value, keep := DropKeys("token").Redact("token", "abc"); keep || value != nil {
		t.Fatalf("DropKeys result = (%v, %v), want nil and dropped", value, keep)
	}
}

func TestDefaultAndComposedRedactors(t *testing.T) {
	if value, keep := DefaultRedactor().Redact("authorization", "Bearer secret"); !keep || value == "Bearer secret" {
		t.Fatalf("DefaultRedactor result = (%v, %v)", value, keep)
	}
	composed := ComposeRedactors(DropKeys("drop"), RedactKeys("mask"))
	if value, keep := composed.Redact("drop", "value"); keep || value != nil {
		t.Fatalf("composed drop result = (%v, %v)", value, keep)
	}
	if value, keep := composed.Redact("mask", "value"); !keep || value == "value" {
		t.Fatalf("composed redact result = (%v, %v)", value, keep)
	}
	if value, keep := composed.Redact("other", "value"); !keep || value != "value" {
		t.Fatalf("composed unmatched result = (%v, %v)", value, keep)
	}
	var nilRedactor core.Redactor
	if value, keep := ComposeRedactors(nilRedactor).Redact("field", "value"); !keep || value != "value" {
		t.Fatalf("nil composed result = (%v, %v)", value, keep)
	}
}
