package nethttp

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/astraive/loxa-go"
)

// Config configures the net/http middleware.
type Config struct {
	Event string
	// RouteExtractor may provide a route template (e.g. "/users/{id}") for a request.
	RouteExtractor func(*http.Request) string
	// TrustForwardedFor controls whether X-Forwarded-For (or ForwardedForHeader) is trusted.
	// Default is false to avoid spoofed client IP logging.
	TrustForwardedFor bool
	// ForwardedForHeader optionally overrides the forwarded-for header name.
	// Default: X-Forwarded-For.
	ForwardedForHeader string
	// HeaderAttrs controls which inbound request headers are copied into attrs as
	// "http.header.<normalized-name>".
	HeaderAttrs []string
}

// Middleware starts a canonical event at request start and emits it on completion.
func Middleware(cfg ...Config) func(http.Handler) http.Handler {
	c := Config{}
	if len(cfg) > 0 {
		c = cfg[0]
	}
	return MiddlewareWithConfig(c)
}

// MiddlewareWithConfig starts a canonical event at request start and emits it on completion.
func MiddlewareWithConfig(cfg Config) func(http.Handler) http.Handler {
	eventName := cfg.Event
	if eventName == "" {
		eventName = "http.request"
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			route := ""
			if cfg.RouteExtractor != nil {
				route = cfg.RouteExtractor(r)
			}

			requestID := r.Header.Get("X-Request-ID")
			traceID := r.Header.Get("X-Trace-ID")
			requestBytes := r.ContentLength
			if requestBytes < 0 {
				requestBytes = 0
			}

			customAttrs := []loxa.Attr{
				loxa.String("scheme", schemeFromRequest(r)),
				loxa.String("user_agent", r.UserAgent()),
				loxa.String("remote_ip", clientIP(r, cfg)),
				loxa.Int64("request_bytes", requestBytes),
			}
			customAttrs = append(customAttrs, selectedHeaderAttrs(r, cfg.HeaderAttrs)...)

			ctx := loxa.StartHTTPEvent(r.Context(), loxa.Params{
				Event:     eventName,
				Kind:      "http",
				Method:    r.Method,
				Path:      r.URL.Path,
				Route:     route,
				Host:      r.Host,
				RequestID: requestID,
				TraceID:   traceID,
				Custom:    customAttrs,
			})

			rw, state := newResponseWriter(w)

			defer func() {
				if rec := recover(); rec != nil {
					if !state.wroteHeader {
						http.Error(rw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					}
					loxa.FinishError(ctx, panicErr{value: rec},
						loxa.Int("status_code", state.statusCode),
						loxa.Int64("response_bytes", int64(state.bytes)),
					)
					_ = loxa.Emit(ctx)
					return
				}

				loxa.Finish(ctx, "success",
					loxa.Int("status_code", state.statusCode),
					loxa.Int64("response_bytes", int64(state.bytes)),
				)
				_ = loxa.Emit(ctx)
			}()

			next.ServeHTTP(rw, r.WithContext(ctx))
		})
	}
}

type panicErr struct {
	value any
}

func (e panicErr) Error() string {
	return fmt.Sprintf("panic recovered: %v", e.value)
}

func schemeFromRequest(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func clientIP(r *http.Request, cfg Config) string {
	if cfg.TrustForwardedFor {
		header := cfg.ForwardedForHeader
		if header == "" {
			header = "X-Forwarded-For"
		}
		if xff := r.Header.Get(header); xff != "" {
			if ip := firstForwardedIP(xff); ip != "" {
				return ip
			}
		}
	}
	return remoteAddrIP(r.RemoteAddr)
}

func firstForwardedIP(xff string) string {
	for _, part := range strings.Split(xff, ",") {
		candidate := strings.TrimSpace(part)
		if candidate == "" {
			continue
		}
		if ip := parseForwardedIPCandidate(candidate); ip != "" {
			return ip
		}
	}
	return ""
}

func parseForwardedIPCandidate(candidate string) string {
	candidate = strings.TrimSpace(candidate)
	candidate = strings.Trim(candidate, "\"")
	if idx := strings.Index(candidate, ";"); idx >= 0 {
		candidate = candidate[:idx]
	}
	candidate = strings.TrimSpace(candidate)
	if strings.HasPrefix(strings.ToLower(candidate), "for=") {
		candidate = strings.TrimSpace(candidate[4:])
		candidate = strings.Trim(candidate, "\"")
	}
	if candidate == "" || strings.EqualFold(candidate, "unknown") {
		return ""
	}
	if ip := net.ParseIP(candidate); ip != nil {
		return ip.String()
	}
	if host, _, err := net.SplitHostPort(candidate); err == nil {
		if ip := net.ParseIP(host); ip != nil {
			return ip.String()
		}
	}
	return ""
}

func remoteAddrIP(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = strings.TrimSpace(addr)
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	return host
}

func selectedHeaderAttrs(r *http.Request, headers []string) []loxa.Attr {
	if len(headers) == 0 {
		return nil
	}
	attrs := make([]loxa.Attr, 0, len(headers))
	for _, h := range headers {
		name := strings.TrimSpace(h)
		if name == "" {
			continue
		}
		value := strings.TrimSpace(r.Header.Get(name))
		if value == "" {
			continue
		}
		// normalize header name to attr key
		key := strings.ToLower(strings.ReplaceAll(name, "_", "-"))
		// validate key (allow lowercase letters, numbers, dot, hyphen)
		if !isValidAttrKey(key) {
			// skip invalid header names
			continue
		}
		attrs = append(attrs, loxa.String("http.header."+key, value))
	}
	return attrs
}

func isValidAttrKey(k string) bool {
	if len(k) == 0 || len(k) > 256 {
		return false
	}
	for _, ch := range k {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '.' || ch == '-' {
			continue
		}
		return false
	}
	return true
}
