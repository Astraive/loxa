package libs_test

import (
	"context"
	"testing"

	"github.com/astraive/loza/sdks/go/src/libs"
)

func TestFlushAllAndCloseAllSkipNilAndReturnNilOnSuccess(t *testing.T) {
	ctx := context.Background()
	if err := libs.FlushAll(ctx, nil, testFlusher{}); err != nil {
		t.Fatalf("FlushAll success = %v", err)
	}
	if err := libs.CloseAll(ctx, nil, testCloser{}); err != nil {
		t.Fatalf("CloseAll success = %v", err)
	}
	if err := libs.FlushAll(ctx); err != nil {
		t.Fatalf("FlushAll empty = %v", err)
	}
	if err := libs.CloseAll(ctx); err != nil {
		t.Fatalf("CloseAll empty = %v", err)
	}
}
