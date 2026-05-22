package otel

import (
	"context"
	"strings"

	"github.com/astraive/loxa/sdks/go"
	"go.opentelemetry.io/otel/baggage"
)

// BaggageAttrs extracts allow-listed baggage keys into loxa attrs.
func BaggageAttrs(ctx context.Context, allowlist ...string) []loxa.Attr {
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

	out := make([]loxa.Attr, 0, len(allowlist))
	for _, m := range bg.Members() {
		if _, ok := set[m.Key()]; ok {
			out = append(out, loxa.String("baggage."+m.Key(), m.Value()))
		}
	}
	return out
}

// EnrichBaggage extracts allow-listed baggage keys and appends them to an active LOXA event.
func EnrichBaggage(ctx context.Context, allowlist ...string) {
	attrs := BaggageAttrs(ctx, allowlist...)
	if len(attrs) == 0 {
		return
	}
	loxa.Enrich(ctx, attrs...)
}
