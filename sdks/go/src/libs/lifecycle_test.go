package libs_test

import (
	"context"
	"errors"
	"testing"

	"github.com/astraive/loxa-go/src/libs"
)

type testFlusher struct{ err error }

func (f testFlusher) Flush(context.Context) error { return f.err }

type testCloser struct{ err error }

func (c testCloser) Close(context.Context) error { return c.err }

func TestFlushAllAndCloseAll(t *testing.T) {
	ctx := context.Background()
	flushErr := errors.New("flush failed")
	closeErr := errors.New("close failed")

	if err := libs.FlushAll(ctx, testFlusher{}, testFlusher{err: flushErr}); !errors.Is(err, flushErr) {
		t.Fatalf("expected flush error, got %v", err)
	}
	if err := libs.CloseAll(ctx, testCloser{}, testCloser{err: closeErr}); !errors.Is(err, closeErr) {
		t.Fatalf("expected close error, got %v", err)
	}
}
