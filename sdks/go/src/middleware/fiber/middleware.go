package fiber

import (
	"fmt"

	"github.com/astraive/loza/sdks/go"
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
		ctx := loza.StartEvent(c.UserContext(), loza.Params{
			Event:  eventName,
			Method: c.Method(),
			Path:   c.Path(),
		})
		c.SetUserContext(ctx)

		defer func() {
			if rec := recover(); rec != nil {
				if !loza.PanicRecoveryEnabled() {
					panic(rec)
				}
				if c.Response().StatusCode() < fiberpkg.StatusBadRequest {
					c.Status(fiberpkg.StatusInternalServerError)
				}
				err = panicErr{value: rec}
				loza.FinishError(ctx, err, loza.Int("status_code", c.Response().StatusCode()))
				_ = loza.Emit(ctx)
			}
		}()

		err = c.Next()
		route := ""
		if c.Route() != nil {
			route = c.Route().Path
		}
		if route != "" {
			loza.Enrich(ctx, loza.String("route", route))
		}
		if err != nil {
			loza.FinishError(ctx, err, loza.Int("status_code", c.Response().StatusCode()))
		} else {
			loza.Finish(ctx, "success", loza.Int("status_code", c.Response().StatusCode()))
		}
		_ = loza.Emit(ctx)
		return err
	}
}

type panicErr struct {
	value any
}

func (e panicErr) Error() string {
	return fmt.Sprintf("panic recovered: %v", e.value)
}
