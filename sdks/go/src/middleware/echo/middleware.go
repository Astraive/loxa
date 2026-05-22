package echo

import (
	"fmt"
	"net/http"

	"github.com/astraive/loxa/sdks/go"
	echopkg "github.com/labstack/echo/v4"
)

// Config controls echo middleware behavior.
type Config struct {
	Event string
}

// Middleware starts and emits canonical events for echo handlers.
func Middleware(cfg ...Config) echopkg.MiddlewareFunc {
	c := Config{}
	if len(cfg) > 0 {
		c = cfg[0]
	}
	return MiddlewareWithConfig(c)
}

// MiddlewareWithConfig starts and emits canonical events for echo handlers.
func MiddlewareWithConfig(cfg Config) echopkg.MiddlewareFunc {
	eventName := cfg.Event
	if eventName == "" {
		eventName = "http.request"
	}

	return func(next echopkg.HandlerFunc) echopkg.HandlerFunc {
		return func(c echopkg.Context) (err error) {
			req := c.Request()
			ctx := loxa.StartEvent(req.Context(), loxa.Params{
				Event:  eventName,
				Method: req.Method,
				Path:   req.URL.Path,
				Route:  c.Path(),
			})
			c.SetRequest(req.WithContext(ctx))

			defer func() {
				if rec := recover(); rec != nil {
					if !loxa.PanicRecoveryEnabled() {
						panic(rec)
					}
					if !c.Response().Committed {
						_ = c.NoContent(http.StatusInternalServerError)
					}
					err = panicErr{value: rec}
					loxa.FinishError(ctx, err, loxa.Int("status_code", c.Response().Status))
					_ = loxa.Emit(ctx)
				}
			}()

			err = next(c)
			if err != nil {
				c.Error(err)
				loxa.FinishError(ctx, err, loxa.Int("status_code", c.Response().Status))
			} else {
				loxa.Finish(ctx, "success", loxa.Int("status_code", c.Response().Status))
			}
			_ = loxa.Emit(ctx)
			return err
		}
	}
}

type panicErr struct {
	value any
}

func (e panicErr) Error() string {
	return fmt.Sprintf("panic recovered: %v", e.value)
}
