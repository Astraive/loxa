package core

import (
	"context"
	"fmt"
)

// Operation is the unit of work wrapped by core lifecycle helpers.
type Operation func(ctx context.Context) error

// Run wraps an operation in the canonical event lifecycle.
func Run(ctx context.Context, params Params, op Operation, finishAttrs ...Attr) error {
	if op == nil {
		return fmt.Errorf("core: operation is nil")
	}
	return RunEvent(ctx, params, func(runCtx context.Context) error {
		return op(runCtx)
	}, finishAttrs...)
}

// RunHTTP wraps an operation in an HTTP canonical event lifecycle.
func RunHTTPOp(ctx context.Context, params Params, op Operation, finishAttrs ...Attr) error {
	if op == nil {
		return fmt.Errorf("core: operation is nil")
	}
	return RunHTTP(ctx, params, func(runCtx context.Context) error {
		return op(runCtx)
	}, finishAttrs...)
}

// RunJob wraps an operation in a job canonical event lifecycle.
func RunJobOp(ctx context.Context, params Params, op Operation, finishAttrs ...Attr) error {
	if op == nil {
		return fmt.Errorf("core: operation is nil")
	}
	return RunJob(ctx, params, func(runCtx context.Context) error {
		return op(runCtx)
	}, finishAttrs...)
}
