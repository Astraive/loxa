package core

import (
	"context"
	"fmt"
	"runtime/debug"
)

// EventFunc runs application work while the canonical event is active.
// Return a non-nil error to mark the event as failed.
type EventFunc func(ctx context.Context) error

// RunEvent wraps an operation in the canonical lifecycle:
// StartEvent -> fn -> Finish/FinishError -> Emit.
func RunEvent(ctx context.Context, params Params, fn EventFunc, finishAttrs ...Attr) error {
	return runEventWithStarter(ctx, params, fn, func(ctx context.Context, params Params) context.Context {
		return Default().StartEvent(ctx, params)
	}, finishAttrs...)
}

// RunHTTP wraps an operation in an HTTP canonical event lifecycle.
func RunHTTP(ctx context.Context, params Params, fn EventFunc, finishAttrs ...Attr) error {
	return runEventWithStarter(ctx, params, fn, func(ctx context.Context, params Params) context.Context {
		if params.Kind == "" {
			params.Kind = "http"
		}
		if params.Event == "" {
			params.Event = "http.request"
		}
		return Default().StartEvent(ctx, params)
	}, finishAttrs...)
}

// RunJob wraps an operation in a job canonical event lifecycle.
func RunJob(ctx context.Context, params Params, fn EventFunc, finishAttrs ...Attr) error {
	return runEventWithStarter(ctx, params, fn, func(ctx context.Context, params Params) context.Context {
		if params.Kind == "" {
			params.Kind = "job"
		}
		if params.Event == "" {
			params.Event = "job.run"
		}
		return Default().StartEvent(ctx, params)
	}, finishAttrs...)
}

// RunQueue wraps an operation in a queue canonical event lifecycle.
func RunQueue(ctx context.Context, params Params, fn EventFunc, finishAttrs ...Attr) error {
	return runEventWithStarter(ctx, params, fn, func(ctx context.Context, params Params) context.Context {
		if params.Kind == "" {
			params.Kind = "queue"
		}
		if params.Event == "" {
			params.Event = "queue.process"
		}
		return Default().StartEvent(ctx, params)
	}, finishAttrs...)
}

// RunCLI wraps an operation in a CLI canonical event lifecycle.
func RunCLI(ctx context.Context, params Params, fn EventFunc, finishAttrs ...Attr) error {
	return runEventWithStarter(ctx, params, fn, func(ctx context.Context, params Params) context.Context {
		if params.Kind == "" {
			params.Kind = "cli"
		}
		if params.Event == "" {
			params.Event = "cli.run"
		}
		return Default().StartEvent(ctx, params)
	}, finishAttrs...)
}

// RunCron wraps an operation in a cron canonical event lifecycle.
func RunCron(ctx context.Context, params Params, fn EventFunc, finishAttrs ...Attr) error {
	return runEventWithStarter(ctx, params, fn, func(ctx context.Context, params Params) context.Context {
		if params.Kind == "" {
			params.Kind = "cron"
		}
		if params.Event == "" {
			params.Event = "cron.run"
		}
		return Default().StartEvent(ctx, params)
	}, finishAttrs...)
}

type eventStarter func(context.Context, Params) context.Context

func runEventWithStarter(ctx context.Context, params Params, fn EventFunc, start eventStarter, finishAttrs ...Attr) (err error) {
	if start == nil {
		return fmt.Errorf("loza: event starter is nil")
	}

	evCtx := start(ctx, params)
	panicRecovery := Default().PanicRecoveryEnabled()
	recovered := false
	defer func() {
		if !recovered && err == nil {
			return
		}

		if err != nil {
			_ = Default().FinishError(evCtx, err, finishAttrs...)
		} else {
			_ = Default().Finish(evCtx, "success", finishAttrs...)
		}

		if emitErr := Default().Emit(evCtx); emitErr != nil {
			if err == nil {
				err = emitErr
			} else {
				err = fmt.Errorf("%w; emit failed: %v", err, emitErr)
			}
		}
	}()
	if panicRecovery {
		defer func() {
			if rec := recover(); rec != nil {
				stack := ""
				if shouldCapturePanicStack(evCtx) {
					stack = string(debug.Stack())
				}
				err = &panicRunError{
					value: rec,
					stack: stack,
				}
				recovered = true
			}
		}()
	}

	if fn == nil {
		recovered = true
		return fmt.Errorf("loza: event function is nil")
	}
	err = fn(evCtx)
	recovered = true
	return err
}

func shouldCapturePanicStack(ctx context.Context) bool {
	ev := loadEvent(ctx)
	if ev == nil {
		return false
	}

	ev.mu.Lock()
	logger := ev.logger
	ev.mu.Unlock()
	if logger == nil {
		return false
	}

	logger.mu.RLock()
	include := logger.cfg.IncludeSource
	logger.mu.RUnlock()
	return include
}

type panicRunError struct {
	value any
	stack string
}

func (e *panicRunError) Error() string {
	if e == nil {
		return "panic recovered"
	}
	return fmt.Sprintf("panic recovered: %v", e.value)
}

func (e *panicRunError) StackTrace() string {
	if e == nil {
		return ""
	}
	return e.stack
}
