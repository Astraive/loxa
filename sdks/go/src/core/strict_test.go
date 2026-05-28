package core

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestStrictConfigValidationMissingService(t *testing.T) {
	sink, _ := MemorySink()
	cfg := Test().WithStrict(true).WithSink(sink)
	_, err := New(cfg)
	if err == nil {
		t.Fatalf("expected strict config validation error")
	}
	var cfgErr *ConfigValidationError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigValidationError, got %T", err)
	}
	if cfgErr.Field != "Service" {
		t.Fatalf("expected Service field error, got %q", cfgErr.Field)
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig wrapper, got %v", err)
	}
}

func TestStrictConfigValidationAsyncKnobs(t *testing.T) {
	sink, _ := MemorySink()
	cfg := Test().
		WithService("svc").
		WithStrict(true).
		WithSink(sink).
		WithAsync(true)
	_, err := New(cfg)
	if err == nil {
		t.Fatalf("expected strict async config validation error")
	}
	var cfgErr *ConfigValidationError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigValidationError, got %T", err)
	}
	if cfgErr.Field != "Async.QueueSize" {
		t.Fatalf("expected Async.QueueSize error, got %q", cfgErr.Field)
	}
}

func TestStrictConfigValidationSecurityNegativeLimit(t *testing.T) {
	sink, _ := MemorySink()
	cfg := Test().
		WithService("svc").
		WithStrict(true).
		WithSink(sink)
	cfg.Security.MaxEventBytes = -1
	_, err := New(cfg)
	if err == nil {
		t.Fatalf("expected strict security validation error")
	}
	var cfgErr *ConfigValidationError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigValidationError, got %T", err)
	}
	if cfgErr.Field != "Security.MaxEventBytes" {
		t.Fatalf("expected Security.MaxEventBytes error, got %q", cfgErr.Field)
	}
}

func TestStrictRejectsMissingServiceOnManualEvent(t *testing.T) {
	sink, _ := MemorySink()
	cfg := Test().WithService("svc").WithStrict(true).WithSink(sink)
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	ev := NewEvent(Params{Event: "checkout.request"})
	if err := l.EmitEvent(ev); err == nil || !strings.Contains(err.Error(), "missing service") {
		t.Fatalf("expected missing service strict error, got %v", err)
	}
}

func TestStrictValidateMethod(t *testing.T) {
	cfg := Test().WithService("svc").WithStrict(true).WithAsyncQueue(32).WithWorkers(2).
		WithAsyncFlushInterval(time.Second).WithAsyncMaxBatchBytes(1024)
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected validate error: %v", err)
	}
}

func TestStrictValidateNoopWhenDisabled(t *testing.T) {
	cfg := Test().WithAsync(true)
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected nil validate error when strict disabled, got %v", err)
	}
}

func TestStrictRejectsMissingEventName(t *testing.T) {
	sink, _ := MemorySink()
	cfg := Test().WithService("svc").WithStrict(true).WithSink(sink)
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	ev := NewEvent(Params{Service: "svc"})
	ev.Event = ""
	if err := l.EmitEvent(ev); err == nil || !strings.Contains(err.Error(), "missing event name") {
		t.Fatalf("expected missing event name strict error, got %v", err)
	}
}

func TestStrictAllowsServiceFromConfig(t *testing.T) {
	sink, _ := MemorySink()
	cfg := Test().WithService("svc").WithStrict(true).WithSink(sink)
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	ctx := l.StartEvent(context.Background(), Params{Event: "checkout.request"})
	_ = l.Enrich(ctx, String("ok", "x"))
	if err := l.Emit(ctx); err != nil {
		t.Fatalf("unexpected strict error: %v", err)
	}
}

func TestStrictRejectsInvalidAttrKey(t *testing.T) {
	sink, _ := MemorySink()
	cfg := Test().WithService("svc").WithStrict(true).WithSink(sink)
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	ctx := l.StartEvent(context.Background(), Params{Event: "checkout.request"})
	_ = l.Enrich(ctx, String("bad-key", "x"))
	if err := l.Emit(ctx); err == nil || !strings.Contains(err.Error(), "invalid attr key") {
		t.Fatalf("expected invalid key error, got %v", err)
	}
}

func TestStrictRejectsCanonicalCollision(t *testing.T) {
	sink, _ := MemorySink()
	cfg := Test().WithService("svc").WithStrict(true).WithSink(sink)
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	ctx := l.StartEvent(context.Background(), Params{Event: "checkout.request"})
	_ = l.Enrich(ctx, String("service", "x"))
	if err := l.Emit(ctx); err == nil || !strings.Contains(err.Error(), "collides with canonical key") {
		t.Fatalf("expected canonical collision error, got %v", err)
	}
}

func TestStrictRejectsUnserializableAny(t *testing.T) {
	sink, _ := MemorySink()
	cfg := Test().WithService("svc").WithStrict(true).WithSink(sink)
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	ctx := l.StartEvent(context.Background(), Params{Event: "checkout.request"})
	_ = l.Enrich(ctx, Any("x", func() {}))
	if err := l.Emit(ctx); err == nil || !strings.Contains(err.Error(), "non-serializable any value") {
		t.Fatalf("expected strict any error, got %v", err)
	}
}

func TestStrictAllowsValidEvent(t *testing.T) {
	sink, store := MemorySink()
	cfg := Test().WithService("svc").WithStrict(true).WithSink(sink)
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	ctx := l.StartEvent(context.Background(), Params{Event: "checkout.request"})
	_ = l.Enrich(ctx, String("user.id", "u-1"), Int("status_code_user", 200))
	if err := l.Emit(ctx); err != nil {
		t.Fatalf("unexpected strict emit error: %v", err)
	}
	if store.Len() != 1 {
		t.Fatalf("expected 1 emitted event, got %d", store.Len())
	}
}

func TestStrictWithValidateEncodedFalse_SkipsSpecContract(t *testing.T) {
	sink, store := MemorySink()
	cfg := Test().
		WithService("svc").
		WithStrict(true).
		WithValidateEncoded(false).
		WithSchema(FlatSchema()).
		WithSink(sink)
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	ctx := l.StartEvent(context.Background(), Params{Event: "checkout.request"})
	_ = l.Enrich(ctx, String("user.id", "u-1"))
	if err := l.Emit(ctx); err != nil {
		t.Fatalf("unexpected error with ValidateEncoded=false: %v", err)
	}
	if store.Len() != 1 {
		t.Fatalf("expected 1 emitted event, got %d", store.Len())
	}
}

func TestStrictWithValidateEncodedTrue_RunsSpecContract(t *testing.T) {
	sink, store := MemorySink()
	cfg := Test().
		WithService("svc").
		WithStrict(true).
		WithSink(sink)
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	ctx := l.StartEvent(context.Background(), Params{Event: "checkout.request"})
	_ = l.Enrich(ctx, String("user.id", "u-1"))
	if err := l.Emit(ctx); err != nil {
		t.Fatalf("unexpected error with default ValidateEncoded: %v", err)
	}
	if store.Len() != 1 {
		t.Fatalf("expected 1 emitted event, got %d", store.Len())
	}
}

func TestStrictWithCustomSchemaAndValidateEncodedFalse(t *testing.T) {
	sink, store := MemorySink()
	cfg := Test().
		WithService("svc").
		WithStrict(true).
		WithValidateEncoded(false).
		WithSchema(ECSchema()).
		WithSink(sink)
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	ctx := l.StartEvent(context.Background(), Params{Event: "checkout.request"})
	_ = l.Enrich(ctx, String("user.id", "u-1"))
	if err := l.Emit(ctx); err != nil {
		t.Fatalf("unexpected error with ECSchema + ValidateEncoded=false: %v", err)
	}
	if store.Len() != 1 {
		t.Fatalf("expected 1 emitted event, got %d", store.Len())
	}
}

func TestWithValidateEncodedConfigOption(t *testing.T) {
	cfg := Test().WithService("svc").WithStrict(true).WithValidateEncoded(false)
	if cfg.ValidateEncoded {
		t.Fatalf("expected ValidateEncoded=false after WithValidateEncoded(false)")
	}
	if !cfg.codeSetValidateEncoded {
		t.Fatalf("expected codeSetValidateEncoded=true after WithValidateEncoded(false)")
	}

	cfg2 := Test().WithService("svc").WithStrict(true).WithValidateEncoded(true)
	if !cfg2.ValidateEncoded {
		t.Fatalf("expected ValidateEncoded=true after WithValidateEncoded(true)")
	}
}
