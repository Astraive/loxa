package chi

import (
	"net/http"

	"github.com/astraive/loxa/sdks/go"
	"github.com/astraive/loxa/sdks/go/src/middleware/nethttp"
	"github.com/go-chi/chi/v5"
)

// Config mirrors net/http middleware configuration.
type Config = nethttp.Config

// Middleware adapts the net/http middleware and enriches route from chi route context.
func Middleware(cfg ...Config) func(http.Handler) http.Handler {
	c := Config{}
	if len(cfg) > 0 {
		c = cfg[0]
	}
	return MiddlewareWithConfig(c)
}

// MiddlewareWithConfig adapts net/http middleware and enriches route from chi route context.
func MiddlewareWithConfig(cfg Config) func(http.Handler) http.Handler {
	fallbackExtractor := cfg.RouteExtractor
	cfg.RouteExtractor = nil
	adapter := nethttp.MiddlewareWithConfig(cfg)

	return func(next http.Handler) http.Handler {
		return adapter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)

			route := ""
			if rc := chi.RouteContext(r.Context()); rc != nil {
				route = rc.RoutePattern()
			}
			if route == "" && fallbackExtractor != nil {
				route = fallbackExtractor(r)
			}
			if route != "" {
				loxa.Set(r.Context(), loxa.Route(route))
			}
		}))
	}
}
