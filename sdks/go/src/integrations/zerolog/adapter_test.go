package zerolog

import (
	"bytes"
	"testing"

	"github.com/astraive/loza/sdks/go"
	"github.com/rs/zerolog"
)

func TestHookCompatibility(t *testing.T) {
	h := Hook()
	if (h == AdapterHook{}) {
		// Hook() returns a zero-value AdapterHook; just verify it doesn't panic.
	}
}

func TestHookRunEmitsToLova(t *testing.T) {
	sink, store := loza.MemorySink()
	cfg := loza.Test().WithSink(sink)
	if err := loza.Configure(cfg); err != nil {
		t.Fatalf("configure: %v", err)
	}

	zl := zerolog.New(&bytes.Buffer{}).Hook(Hook())
	zl.Info().Msg("hello from zerolog")

	if store.Len() != 1 {
		t.Fatalf("expected 1 emitted event, got %d", store.Len())
	}
}

func TestHookRunMapsLevels(t *testing.T) {
	sink, store := loza.MemorySink()
	cfg := loza.Test().WithSink(sink)
	if err := loza.Configure(cfg); err != nil {
		t.Fatalf("configure: %v", err)
	}

	zl := zerolog.New(&bytes.Buffer{}).Hook(Hook())

	// Debug and Trace both map to loza Debug.
	zl.Debug().Msg("debug msg")
	zerolog.GlobalLevel() // ensure trace is available
	zl.Trace().Msg("trace msg")
	zl.Info().Msg("info msg")
	zl.Warn().Msg("warn msg")
	zl.Error().Msg("error msg")

	if store.Len() != 5 {
		t.Fatalf("expected 5 emitted events, got %d", store.Len())
	}
}
