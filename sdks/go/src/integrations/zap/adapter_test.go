package zap

import (
	"bytes"
	"context"
	"testing"
	"time"

	loza "github.com/astraive/loza/sdks/go"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestAdapterCoreWritesAllLevelsToWrappedCoreAndLoza(t *testing.T) {
	sink, store := loza.MemorySink()
	if err := loza.Configure(loza.Test().WithSink(sink)); err != nil {
		t.Fatalf("configure: %v", err)
	}
	t.Cleanup(func() { _ = loza.Shutdown(context.Background()) })
	var output bytes.Buffer
	base := zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), zapcore.AddSync(&output), zapcore.DebugLevel)
	adapter := Core(base)
	if adapter == nil || NewCore(base) == nil {
		t.Fatal("Core constructor returned nil")
	}
	for _, level := range []zapcore.Level{zapcore.DebugLevel, zapcore.InfoLevel, zapcore.WarnLevel, zapcore.ErrorLevel} {
		ent := zapcore.Entry{Level: level, Message: "zap message", Time: time.Unix(1, 0)}
		if err := adapter.Write(ent, []zapcore.Field{zap.String("key", "value")}); err != nil {
			t.Fatalf("Write %v: %v", level, err)
		}
	}
	if output.Len() == 0 {
		t.Fatal("wrapped core received no output")
	}
	if err := loza.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if store.Len() != 4 {
		t.Fatalf("loza events = %d, want 4", store.Len())
	}
}
