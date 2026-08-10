package gin

import (
	"fmt"
	"net/http"

	"github.com/astraive/loza/sdks/go"
	ginpkg "github.com/gin-gonic/gin"
)

// Config controls gin middleware behavior.
type Config struct {
	Event string
}

// Middleware starts and emits canonical events for gin handlers.
func Middleware(cfg ...Config) ginpkg.HandlerFunc {
	c := Config{}
	if len(cfg) > 0 {
		c = cfg[0]
	}
	return MiddlewareWithConfig(c)
}

// MiddlewareWithConfig starts and emits canonical events for gin handlers.
func MiddlewareWithConfig(cfg Config) ginpkg.HandlerFunc {
	eventName := cfg.Event
	if eventName == "" {
		eventName = "http.request"
	}

	return func(c *ginpkg.Context) {
		ctx := loza.StartEvent(c.Request.Context(), loza.Params{
			Event:  eventName,
			Method: c.Request.Method,
			Path:   c.Request.URL.Path,
			Route:  c.FullPath(),
		})
		c.Request = c.Request.WithContext(ctx)

		defer func() {
			if rec := recover(); rec != nil {
				if loza.PanicRecoveryEnabled() {
					if c.Writer.Status() < http.StatusBadRequest {
						c.AbortWithStatus(http.StatusInternalServerError)
					}
					loza.FinishError(ctx, panicErr{value: rec}, loza.Int("status_code", c.Writer.Status()))
					_ = loza.Emit(ctx)
					return
				}
				panic(rec)
			}
			if len(c.Errors) > 0 {
				loza.FinishError(ctx, c.Errors.Last(), loza.Int("status_code", c.Writer.Status()))
			} else {
				loza.Finish(ctx, "success", loza.Int("status_code", c.Writer.Status()))
			}
			_ = loza.Emit(ctx)
		}()

		c.Next()
	}
}

type panicErr struct {
	value any
}

func (e panicErr) Error() string {
	return fmt.Sprintf("panic recovered: %v", e.value)
}
