package conformance

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/astraive/loza/sdks/go"
)

type fixture struct {
	Event   string            `json:"event"`
	Outcome string            `json:"outcome"`
	Attrs   map[string]string `json:"attrs"`
}

func TestConformanceFixtures(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("cases", "*.json"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no fixture files found")
	}

	sink, store := loza.MemorySink()
	cfg := loza.Test().WithSink(sink)
	if err := loza.Configure(cfg); err != nil {
		t.Fatalf("configure: %v", err)
	}

	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var fx fixture
			if err := json.Unmarshal(raw, &fx); err != nil {
				t.Fatalf("unmarshal fixture: %v", err)
			}

			ctx := loza.StartEvent(context.Background(), loza.Params{Event: fx.Event})
			var attrs []loza.Attr
			for k, v := range fx.Attrs {
				attrs = append(attrs, loza.String(k, v))
			}
			_ = loza.Enrich(ctx, attrs...)
			_ = loza.Finish(ctx, fx.Outcome)
			if err := loza.Emit(ctx); err != nil {
				t.Fatalf("emit: %v", err)
			}
		})
	}

	if store.Len() == 0 {
		t.Fatalf("expected conformance fixtures to emit events")
	}
}
