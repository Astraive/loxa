package conformance

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/astraive/loza/sdks/go"
)

func TestSamplingAndPolicyHelpers(t *testing.T) {
	sink, store := loza.MemorySink()
	cfg := loza.Test().
		WithSink(sink).
		WithSampler(loza.SampleByEvent("verification.sampled")).
		WithRedactor(loza.ComposeRedactors(loza.DefaultRedactor(), loza.RedactKeys("password")))
	if err := loza.Configure(cfg); err != nil {
		t.Fatalf("configure: %v", err)
	}

	ctx := loza.StartEvent(context.Background(), loza.Params{Event: "verification.sampled"})
	if err := loza.Enrich(ctx, loza.String("password", "secret123")); err != nil {
		t.Fatalf("enrich: %v", err)
	}
	if err := loza.Finish(ctx, "success"); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if err := loza.Emit(ctx); err != nil {
		t.Fatalf("emit: %v", err)
	}
	raw := store.Raw()
	if len(raw) == 0 {
		t.Fatalf("expected emitted sampled event")
	}
	if got := string(raw[0]); !json.Valid(raw[0]) || !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("expected encoded payload to contain redacted value, got %s", got)
	}
	if loza.SampleByOutcome("error") == nil {
		t.Fatalf("expected sample-by-outcome sampler")
	}
	if loza.AllowFields("allowed") == nil {
		t.Fatalf("expected allow-fields sampler")
	}
	if loza.BlockFields("blocked") == nil {
		t.Fatalf("expected block-fields sampler")
	}
}
