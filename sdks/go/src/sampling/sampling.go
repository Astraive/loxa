package sampling

import (
	"time"

	"github.com/astraive/loza/sdks/go/src/core"
)

// SampleAll keeps every event.
func SampleAll() core.Sampler { return core.SampleAll() }

// SampleNone drops every event.
func SampleNone() core.Sampler { return core.SampleNone() }

// SampleRandom keeps events with the given probability [0.0, 1.0].
func SampleRandom(probability float64) core.Sampler { return core.SampleRandom(probability) }

// SampleRateLimited returns a sampler that limits to rate events per window.
func SampleRateLimited(rate float64, window time.Duration) core.Sampler {
	return core.SampleRateLimited(rate, window)
}

// SampleErrors keeps events where level >= Error.
func SampleErrors() core.Sampler { return core.SampleErrors() }

// SampleSlowRequests keeps events slower than threshold.
func SampleSlowRequests(threshold any) core.Sampler {
	return core.SampleSlowRequests(threshold)
}

// SampleStatusCodes keeps events matching the given HTTP status codes.
func SampleStatusCodes(codes ...int) core.Sampler { return core.SampleStatusCodes(codes...) }

// SampleRoutes keeps events matching the given route patterns.
func SampleRoutes(patterns ...string) core.Sampler { return core.SampleRoutes(patterns...) }

// SampleUsers keeps events for the given user IDs.
func SampleUsers(userIDs ...string) core.Sampler { return core.SampleUsers(userIDs...) }

// SampleTenants keeps events for the given tenant IDs.
func SampleTenants(tenantIDs ...string) core.Sampler { return core.SampleTenants(tenantIDs...) }

// AnySampler keeps an event if any sub-sampler keeps it.
func AnySampler(samplers ...core.Sampler) core.Sampler { return core.AnySampler(samplers...) }

// AllSampler keeps an event only if all sub-samplers keep it.
func AllSampler(samplers ...core.Sampler) core.Sampler { return core.AllSampler(samplers...) }

// NotSampler inverts the decision of the wrapped sampler.
func NotSampler(s core.Sampler) core.Sampler { return core.NotSampler(s) }
