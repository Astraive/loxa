package otel

import (
	"context"
	"strings"

	"github.com/astraive/loza/sdks/go"
	"go.opentelemetry.io/otel/baggage"
)

// BaggageAttrs extracts allow-listed baggage keys into loza attrs.
func BaggageAttrs(ctx context.Context, allowlist ...string) []loza.Attr {
	if ctx == nil || len(allowlist) == 0 {
		return nil
	}
	bg := baggage.FromContext(ctx)

	set := map[string]struct{}{}
	for _, k := range allowlist {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		set[k] = struct{}{}
	}
	if len(set) == 0 {
		return nil
	}

	out := make([]loza.Attr, 0, len(allowlist))
	for _, m := range bg.Members() {
		if _, ok := set[m.Key()]; ok {
			out = append(out, loza.String("baggage."+m.Key(), m.Value()))
		}
	}
	return out
}

// EnrichBaggage extracts allow-listed baggage keys and appends them to an active LOZA event.
func EnrichBaggage(ctx context.Context, allowlist ...string) {
	attrs := BaggageAttrs(ctx, allowlist...)
	if len(attrs) == 0 {
		return
	}
	loza.Enrich(ctx, attrs...)
}
