package sampling

import (
	"testing"
	"time"

	"github.com/astraive/loza/sdks/go/src/core"
)

func TestSamplingConstructorsMatchEventAttributes(t *testing.T) {
	ev := &core.Event{
		Event:      "checkout",
		Outcome:    "success",
		StatusCode: 200,
		DurationMS: 150,
		Route:      "/checkout",
		Path:       "/checkout",
		Attrs: []core.Attr{
			core.String("user.id", "u-1"),
			core.String("tenant_id", "tenant-1"),
			core.String("feature.preview", "on"),
			core.String("http.header.x-debug", "yes, true"),
			core.String("custom", "value"),
		},
	}
	if !SampleAll().ShouldSample(ev) || SampleNone().ShouldSample(ev) {
		t.Fatal("all/none samplers returned unexpected decision")
	}
	if !SampleErrors().ShouldSample(&core.Event{Level: core.LevelError}) {
		t.Fatal("error sampler did not match error-level event")
	}
	if !SampleRandom(1).ShouldSample(ev) || SampleRandom(0).ShouldSample(ev) {
		t.Fatal("random boundary samplers returned unexpected decision")
	}
	if !SampleSlowRequests(100).ShouldSample(ev) || !SampleSlowRequests(100*time.Millisecond).ShouldSample(ev) {
		t.Fatal("slow request sampler did not match duration")
	}
	if !SampleUsers("u-1").ShouldSample(ev) || !SampleTenants("tenant-1").ShouldSample(ev) {
		t.Fatal("user/tenant sampler did not match event")
	}
	if !SampleFeatureFlag("preview", "on").ShouldSample(ev) {
		t.Fatal("feature flag sampler did not match event")
	}
	if !SampleStatusCodes(200).ShouldSample(ev) || !SampleRoutes("/checkout").ShouldSample(ev) {
		t.Fatal("status/route sampler did not match event")
	}
	if !SampleByHeader("X_Debug", "true").ShouldSample(ev) {
		t.Fatal("header sampler did not match comma-separated value")
	}
	if !SampleByEvent("checkout").ShouldSample(ev) || !SampleByOutcome("success").ShouldSample(ev) {
		t.Fatal("event/outcome sampler did not match event")
	}
	if !AllowFields("custom").ShouldSample(ev) || !BlockFields("forbidden").ShouldSample(ev) {
		t.Fatal("allow/block field samplers returned unexpected decision")
	}
}

func TestSamplingBoundaryAndCompositionDecisions(t *testing.T) {
	if SampleSlowRequests("invalid").ShouldSample(&core.Event{}) {
		t.Fatal("invalid slow threshold should drop")
	}
	if !SampleSlowRequests(0).ShouldSample(&core.Event{}) {
		t.Fatal("non-positive slow threshold should keep")
	}
	if SampleByHeader("", "x").ShouldSample(&core.Event{}) {
		t.Fatal("empty header should drop")
	}
	if SampleUsers().ShouldSample(nil) || SampleTenants().ShouldSample(nil) {
		t.Fatal("empty user/tenant samplers should drop nil event")
	}
	if !AnySampler(SampleNone(), SampleAll()).ShouldSample(nil) {
		t.Fatal("AnySampler should keep when one sampler keeps")
	}
	if !AllSampler(SampleAll(), nil).ShouldSample(nil) {
		t.Fatal("AllSampler should ignore nil samplers")
	}
	if NotSampler(SampleNone()).ShouldSample(nil) != true {
		t.Fatal("NotSampler should invert wrapped sampler")
	}
	if NotSampler(nil).ShouldSample(nil) {
		t.Fatal("NotSampler(nil) should drop")
	}
	limited := SampleRateLimited(0, time.Second)
	if limited.ShouldSample(nil) {
		t.Fatal("non-positive rate should drop")
	}
}
