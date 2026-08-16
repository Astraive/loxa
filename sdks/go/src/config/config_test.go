package config

import (
	"context"
	"testing"

	"github.com/astraive/loza/sdks/go/src/core"
)

func TestConfigPresetsAndOptions(t *testing.T) {
	production := Production()
	dev := Dev()
	testConfig := Test()
	if production.Environment == "" || dev.Environment == "" || testConfig.Environment == "" {
		t.Fatal("config presets should set environment")
	}
	sink, _ := core.MemorySink()
	custom := ApplyConfig(testConfig,
		WithService("checkout"),
		WithVersion("1.2.3"),
		WithEnvironment("staging"),
		WithSink(sink),
		WithSampler(core.SampleAll()),
		WithSchema(core.FlatSchema()),
		WithRedactor(core.DefaultRedactor()),
		WithAsync(false),
		WithCollectorURL("http://collector.local"),
		WithDSN("loza://collector.example.com/demo?env=staging&service=checkout"),
		WithBatchSize(3),
		WithStrict(true),
		WithEnricher(func(context.Context) []core.Attr { return []core.Attr{core.String("source", "test")} }),
	)
	if custom.Service != "checkout" || custom.Version != "1.2.3" || custom.Environment != "staging" {
		t.Fatalf("custom identity = %+v", custom)
	}
	if len(custom.Sinks) != 1 || custom.Sampler == nil || custom.Schema == nil || custom.Redactor == nil || custom.BatchSize != 3 || !custom.Strict {
		t.Fatalf("options not applied: %+v", custom)
	}
	if custom.Enricher == nil || custom.CollectorURL == "" {
		t.Fatalf("extended options not applied: %+v", custom)
	}
}

func TestConfigNewAndEnvironmentLoading(t *testing.T) {
	logger, err := New(ApplyConfig(core.Config{}, WithService("new")))
	if err != nil || logger == nil {
		t.Fatalf("New valid config = logger %v, error %v", logger, err)
	}
	_ = logger.Shutdown(context.Background())
	cfg := ApplyConfig(Test(), WithService("from-env"))
	loaded := LoadFromEnv(cfg)
	if loaded.Service == "" {
		t.Fatal("LoadFromEnv lost configured service")
	}
}
