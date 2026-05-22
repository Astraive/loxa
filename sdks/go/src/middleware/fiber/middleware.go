package fiber

import (
	"fmt"

	"github.com/astraive/loxa/sdks/go"
	fiberpkg "github.com/gofiber/fiber/v2"
)

// Config controls fiber middleware behavior.
type Config struct {
	Event string
}

// Middleware starts and emits canonical events for fiber handlers.
func Middleware(cfg ...Config) fiberpkg.Handler {
	c := Config{}
	if len(cfg) > 0 {
		c = cfg[0]
	}
	return MiddlewareWithConfig(c)
}

// MiddlewareWithConfig starts and emits canonical events for fiber handlers.
func MiddlewareWithConfig(cfg Config) fiberpkg.Handler {
	eventName := cfg.Event
	if eventName == "" {
		eventName = "http.request"
	}

	return func(c *fiberpkg.Ctx) (err error) {
		ctx := loxa.StartEvent(c.UserContext(), loxa.Params{
			Event:  eventName,
			Method: c.Method(),
			Path:   c.Path(),
		})
		c.SetUserContext(ctx)

		defer func() {
			if rec := recover(); rec != nil {
				if !loxa.PanicRecoveryEnabled() {
					panic(rec)
				}
				if c.Response().StatusCode() < fiberpkg.StatusBadRequest {
					c.Status(fiberpkg.StatusInternalServerError)
				}
				err = panicErr{value: rec}
				loxa.FinishError(ctx, err, loxa.Int("status_code", c.Response().StatusCode()))
				_ = loxa.Emit(ctx)
			}
		}()

		err = c.Next()
		route := ""
		if c.Route() != nil {
			route = c.Route().Path
		}
		if route != "" {
			loxa.Enrich(ctx, loxa.String("route", route))
		}
		if err != nil {
			loxa.FinishError(ctx, err, loxa.Int("status_code", c.Response().StatusCode()))
		} else {
			loxa.Finish(ctx, "success", loxa.Int("status_code", c.Response().StatusCode()))
		}
		_ = loxa.Emit(ctx)
		return err
	}
}

type panicErr struct {
	value any
}

func (e panicErr) Error() string {
	return fmt.Sprintf("panic recovered: %v", e.value)
}
