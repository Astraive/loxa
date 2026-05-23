package core

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"time"
)

// Sampler decides whether an event should be emitted.
type Sampler interface {
	ShouldSample(ev *Event) bool
}

type samplerFunc func(ev *Event) bool

func (f samplerFunc) ShouldSample(ev *Event) bool {
	if f == nil {
		return true
	}
	return f(ev)
}

// SampleAll keeps every event.
func SampleAll() Sampler {
	return samplerFunc(func(_ *Event) bool { return true })
}

// SampleNone drops every event.
func SampleNone() Sampler {
	return samplerFunc(func(_ *Event) bool { return false })
}

// SampleRandom keeps approximately rate of events (0..1).
func SampleRandom(rate float64) Sampler {
	switch {
	case rate <= 0:
		return SampleNone()
	case rate >= 1:
		return SampleAll()
	default:
		return samplerFunc(func(_ *Event) bool {
			return rand.Float64() < rate
		})
	}
}

// SampleErrors keeps only error events.
func SampleErrors() Sampler {
	return samplerFunc(func(ev *Event) bool {
		if ev == nil {
			return false
		}
		ev.MuLock()
		defer ev.MuUnlock()
		return ev.Error != nil || ev.Level >= LevelError || ev.Outcome == "error"
	})
}

// SampleSlowRequests keeps events with duration >= threshold.
// threshold may be time.Duration, int64 (milliseconds), or int (milliseconds).
func SampleSlowRequests(threshold any) Sampler {
	var thresholdMS int64
	switch v := threshold.(type) {
	case time.Duration:
		thresholdMS = v.Milliseconds()
	case int64:
		thresholdMS = v
	case int:
		thresholdMS = int64(v)
	default:
		return SampleNone()
	}
	if thresholdMS <= 0 {
		return SampleAll()
	}
	return samplerFunc(func(ev *Event) bool {
		if ev == nil {
			return false
		}
		ev.MuLock()
		defer ev.MuUnlock()
		return ev.DurationMS >= thresholdMS
	})
}

// SampleUsers keeps events whose user identifier matches one of ids.
// It checks both "user.id" and "user_id".
func SampleUsers(ids ...string) Sampler {
	allow := makeSet(ids...)
	return samplerFunc(func(ev *Event) bool {
		if ev == nil || len(allow) == 0 {
			return false
		}
		ev.MuLock()
		defer ev.MuUnlock()
		id, ok := lookupAttrString(ev.Attrs, "user.id", "user_id")
		return ok && hasSet(allow, id)
	})
}

// SampleTenants keeps events whose tenant identifier matches one of ids.
// It checks both "tenant.id" and "tenant_id".
func SampleTenants(ids ...string) Sampler {
	allow := makeSet(ids...)
	return samplerFunc(func(ev *Event) bool {
		if ev == nil || len(allow) == 0 {
			return false
		}
		ev.MuLock()
		defer ev.MuUnlock()
		id, ok := lookupAttrString(ev.Attrs, "tenant.id", "tenant_id")
		return ok && hasSet(allow, id)
	})
}

// SampleFeatureFlag keeps events where feature/feature_flags.<name> matches value.
func SampleFeatureFlag(name string, value any) Sampler {
	name = strings.TrimSpace(name)
	if name == "" {
		return SampleNone()
	}
	return samplerFunc(func(ev *Event) bool {
		if ev == nil {
			return false
		}
		ev.MuLock()
		defer ev.MuUnlock()
		v, ok := lookupAttrValue(ev.Attrs, "feature."+name)
		if !ok {
			v, ok = lookupAttrValue(ev.Attrs, "feature_flags."+name)
		}
		if !ok {
			return false
		}
		return fmt.Sprintf("%v", v) == fmt.Sprintf("%v", value)
	})
}

// SampleStatusCodes keeps events whose status code matches one of codes.
func SampleStatusCodes(codes ...int) Sampler {
	allow := make(map[int]struct{}, len(codes))
	for _, c := range codes {
		allow[c] = struct{}{}
	}
	return samplerFunc(func(ev *Event) bool {
		if ev == nil || len(allow) == 0 {
			return false
		}
		ev.MuLock()
		defer ev.MuUnlock()
		_, ok := allow[ev.StatusCode]
		return ok
	})
}

// SampleRoutes keeps events whose route or path matches one of routes.
func SampleRoutes(routes ...string) Sampler {
	allow := makeSet(routes...)
	return samplerFunc(func(ev *Event) bool {
		if ev == nil || len(allow) == 0 {
			return false
		}
		ev.MuLock()
		defer ev.MuUnlock()
		return hasSet(allow, ev.Route) || hasSet(allow, ev.Path)
	})
}

// SampleRateLimited keeps at most rate events per window using a token-bucket strategy.
func SampleRateLimited(rate float64, window time.Duration) Sampler {
	if rate <= 0 || window <= 0 {
		return SampleNone()
	}
	return &rateLimitedSampler{
		rate:   rate,
		window: window,
		last:   time.Now(),
		tokens: rate,
	}
}

// SampleByHeader keeps events where a header attr equals value.
// It checks keys:
//   - http.header.<name>
//   - http.headers.<name>
//   - <name>
// where <name> is lower-cased with "_" converted to "-".
func SampleByHeader(header, value string) Sampler {
	h := normalizeHeaderKey(header)
	if h == "" {
		return SampleNone()
	}
	want := strings.TrimSpace(value)
	keys := []string{
		"http.header." + h,
		"http.headers." + h,
		h,
	}
	return samplerFunc(func(ev *Event) bool {
		if ev == nil {
			return false
		}
		ev.MuLock()
		defer ev.MuUnlock()
		got, ok := lookupAttrString(ev.Attrs, keys...)
		if !ok {
			return false
		}
		if want == "" {
			return strings.TrimSpace(got) != ""
		}
		for _, part := range strings.Split(got, ",") {
			if strings.EqualFold(strings.TrimSpace(part), want) {
				return true
			}
		}
		return false
	})
}

// AnySampler keeps an event if any sampler keeps it.
func AnySampler(samplers ...Sampler) Sampler {
	return samplerFunc(func(ev *Event) bool {
		for _, s := range samplers {
			if s != nil && s.ShouldSample(ev) {
				return true
			}
		}
		return false
	})
}

// AllSampler keeps an event only if all samplers keep it.
func AllSampler(samplers ...Sampler) Sampler {
	return samplerFunc(func(ev *Event) bool {
		for _, s := range samplers {
			if s != nil && !s.ShouldSample(ev) {
				return false
			}
		}
		return true
	})
}

// NotSampler inverts another sampler.
func NotSampler(s Sampler) Sampler {
	return samplerFunc(func(ev *Event) bool {
		if s == nil {
			return false
		}
		return !s.ShouldSample(ev)
	})
}

type rateLimitedSampler struct {
	rate   float64
	window time.Duration

	mu     sync.Mutex
	tokens float64
	last   time.Time
}

func (s *rateLimitedSampler) ShouldSample(_ *Event) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if s.last.IsZero() {
		s.last = now
	}
	elapsed := now.Sub(s.last)
	s.last = now

	capacity := s.rate
	if capacity < 1 {
		capacity = 1
	}
	s.tokens += elapsed.Seconds() * (s.rate / s.window.Seconds())
	if s.tokens > capacity {
		s.tokens = capacity
	}
	if s.tokens < 1 {
		return false
	}
	s.tokens -= 1
	return true
}

func makeSet(values ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		out[v] = struct{}{}
	}
	return out
}

func hasSet(set map[string]struct{}, v string) bool {
	_, ok := set[v]
	return ok
}

func lookupAttrString(attrs []Attr, keys ...string) (string, bool) {
	for _, key := range keys {
		v, ok := lookupAttrValue(attrs, key)
		if !ok {
			continue
		}
		switch vv := v.(type) {
		case string:
			return vv, true
		case fmt.Stringer:
			return vv.String(), true
		default:
			return fmt.Sprintf("%v", vv), true
		}
	}
	return "", false
}

func lookupAttrValue(attrs []Attr, key string) (any, bool) {
	for _, a := range attrs {
		if a.Key == key {
			return a.Value, true
		}
		if a.Kind != KindGroup {
			continue
		}
		children, ok := a.Value.([]Attr)
		if !ok {
			continue
		}
		prefix := a.Key + "."
		if strings.HasPrefix(key, prefix) {
			if v, ok := lookupAttrValue(children, key[len(prefix):]); ok {
				return v, true
			}
		}
	}
	return nil, false
}

func normalizeHeaderKey(key string) string {
	key = strings.TrimSpace(strings.ToLower(key))
	if key == "" {
		return ""
	}
	key = strings.ReplaceAll(key, "_", "-")
	return key
}

// SampleByEvent keeps events whose event name matches one of names.
func SampleByEvent(names ...string) Sampler {
	allow := makeSet(names...)
	return samplerFunc(func(ev *Event) bool {
		if ev == nil || len(allow) == 0 {
			return false
		}
		ev.MuLock()
		defer ev.MuUnlock()
		return hasSet(allow, ev.Event)
	})
}

// SampleByOutcome keeps events whose outcome matches one of outcomes.
func SampleByOutcome(outcomes ...string) Sampler {
	allow := makeSet(outcomes...)
	return samplerFunc(func(ev *Event) bool {
		if ev == nil || len(allow) == 0 {
			return false
		}
		ev.MuLock()
		defer ev.MuUnlock()
		return hasSet(allow, ev.Outcome)
	})
}

// AllowFields returns a Sampler that keeps events when the attr list contains
// any of the specified keys.
func AllowFields(keys ...string) Sampler {
	allow := makeSet(keys...)
	return samplerFunc(func(ev *Event) bool {
		if ev == nil || len(allow) == 0 {
			return false
		}
		ev.MuLock()
		defer ev.MuUnlock()
		for _, a := range ev.Attrs {
			if hasSet(allow, a.Key) {
				return true
			}
		}
		return false
	})
}

// BlockFields returns a Sampler that drops events when the attr list contains
// any of the specified keys.
func BlockFields(keys ...string) Sampler {
	block := makeSet(keys...)
	return samplerFunc(func(ev *Event) bool {
		if ev == nil || len(block) == 0 {
			return true
		}
		ev.MuLock()
		defer ev.MuUnlock()
		for _, a := range ev.Attrs {
			if hasSet(block, a.Key) {
				return false
			}
		}
		return true
	})
}
