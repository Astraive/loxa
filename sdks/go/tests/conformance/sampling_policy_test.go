package conformance

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/astraive/loxa/sdks/go"
)

func TestSamplingAndPolicyHelpers(t *testing.T) {
	sink, store := loxa.MemorySink()
	cfg := loxa.Test().
		WithSink(sink).
		WithSampler(loxa.SampleByEvent("verification.sampled")).
		WithRedactor(loxa.ComposeRedactors(loxa.DefaultRedactor(), loxa.RedactKeys("password")))
	if err := loxa.Configure(cfg); err != nil {
		t.Fatalf("configure: %v", err)
	}

	ctx := loxa.StartEvent(context.Background(), loxa.Params{Event: "verification.sampled"})
	if err := loxa.Enrich(ctx, loxa.String("password", "secret123")); err != nil {
		t.Fatalf("enrich: %v", err)
	}
	if err := loxa.Finish(ctx, "success"); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if err := loxa.Emit(ctx); err != nil {
		t.Fatalf("emit: %v", err)
	}
	raw := store.Raw()
	if len(raw) == 0 {
		t.Fatalf("expected emitted sampled event")
	}
	if got := string(raw[0]); !json.Valid(raw[0]) || !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("expected encoded payload to contain redacted value, got %s", got)
	}
	if loxa.SampleByOutcome("error") == nil {
		t.Fatalf("expected sample-by-outcome sampler")
	}
	if loxa.AllowFields("allowed") == nil {
		t.Fatalf("expected allow-fields sampler")
	}
	if loxa.BlockFields("blocked") == nil {
		t.Fatalf("expected block-fields sampler")
	}
}
