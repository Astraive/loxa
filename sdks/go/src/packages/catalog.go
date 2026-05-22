package packages

// Middleware returns the import paths for supported middleware adapters.
func Middleware() []string {
	return []string{
		"github.com/astraive/loxa/sdks/go/src/middleware/chi",
		"github.com/astraive/loxa/sdks/go/src/middleware/echo",
		"github.com/astraive/loxa/sdks/go/src/middleware/fiber",
		"github.com/astraive/loxa/sdks/go/src/middleware/gin",
		"github.com/astraive/loxa/sdks/go/src/middleware/grpc",
		"github.com/astraive/loxa/sdks/go/src/middleware/nethttp",
	}
}

// Integrations returns the import paths for supported integration adapters.
func Integrations() []string {
	return []string{
		"github.com/astraive/loxa/sdks/go/src/integrations/otel",
		"github.com/astraive/loxa/sdks/go/src/integrations/slog",
		"github.com/astraive/loxa/sdks/go/src/integrations/zap",
		"github.com/astraive/loxa/sdks/go/src/integrations/zerolog",
	}
}

// Sinks returns the import paths for SDK-owned lightweight sink packages.
func Sinks() []string {
	return []string{
		"github.com/astraive/loxa/sdks/go/src/sinks/httpbatch",
	}
}
