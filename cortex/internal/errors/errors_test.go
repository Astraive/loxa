package errors

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestRetryWithBackoffSucceedsAfterRetries(t *testing.T) {
	ctx := context.Background()
	attempts := 0

	err := RetryWithBackoff(ctx, 3, time.Millisecond, func() error {
		attempts++
		if attempts < 3 {
			return fmt.Errorf("fail")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetryWithBackoffHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := RetryWithBackoff(ctx, 3, time.Millisecond, func() error {
		return fmt.Errorf("fail")
	})
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestTimeoutHelpers(t *testing.T) {
	err := NewTimeoutError(true, context.DeadlineExceeded)
	if !IsTimeout(err) {
		t.Fatal("expected timeout error")
	}
	if IsRetryable(context.Canceled) {
		t.Fatal("context canceled should not be retryable")
	}
	if !IsRetryable(fmt.Errorf("other")) {
		t.Fatal("generic errors should be retryable")
	}
}
