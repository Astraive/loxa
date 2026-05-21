package errors

import (
	"context"
	"fmt"
	"time"
)

func RetryWithBackoff(ctx context.Context, maxAttempts int, initialDelay time.Duration, fn func() error) error {
	delay := initialDelay
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := fn(); err != nil {
			lastErr = err
			if attempt < maxAttempts {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(delay):
				}
				delay = delay * 2
				if delay > 30*time.Second {
					delay = 30 * time.Second
				}
			}
			continue
		}
		return nil
	}

	return fmt.Errorf("retry failed after %d attempts: %w", maxAttempts, lastErr)
}

func IsRetryable(err error) bool {
	if err == context.DeadlineExceeded || err == context.Canceled {
		return false
	}
	return true
}

type TimeoutError struct {
	Partial bool
	Cause   error
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("operation timed out (partial: %v): %v", e.Partial, e.Cause)
}

func (e *TimeoutError) Unwrap() error {
	return e.Cause
}

func IsTimeout(err error) bool {
	_, ok := err.(*TimeoutError)
	return ok
}

func NewTimeoutError(partial bool, cause error) *TimeoutError {
	return &TimeoutError{Partial: partial, Cause: cause}
}