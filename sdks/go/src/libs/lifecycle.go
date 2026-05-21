package libs

import (
	"context"
	"errors"
)

// Flusher is implemented by components that support explicit flush.
type Flusher interface {
	Flush(context.Context) error
}

// Closer is implemented by components that support shutdown/close.
type Closer interface {
	Close(context.Context) error
}

// FlushAll flushes all flushers and returns a joined error when one or more fail.
func FlushAll(ctx context.Context, flushers ...Flusher) error {
	var errs []error
	for _, f := range flushers {
		if f == nil {
			continue
		}
		if err := f.Flush(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// CloseAll closes all closers and returns a joined error when one or more fail.
func CloseAll(ctx context.Context, closers ...Closer) error {
	var errs []error
	for _, c := range closers {
		if c == nil {
			continue
		}
		if err := c.Close(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
