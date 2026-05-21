package cortex

import (
	"github.com/astraive/loxa-go/src/core"
)

// DefaultSchema returns the default LOXA event schema.
func DefaultSchema() core.Schema { return core.DefaultSchema() }

// FlatSchema returns a flat key-value schema.
func FlatSchema() core.Schema { return core.FlatSchema() }

// NestedSchema returns a nested/grouped schema.
func NestedSchema() core.Schema { return core.NestedSchema() }

// OTelLogSchema returns an OpenTelemetry log schema.
func OTelLogSchema() core.Schema { return core.OTelLogSchema() }

// ECSchema returns an Elastic Common Schema mapping.
func ECSchema() core.Schema { return core.ECSchema() }

// DatadogSchema returns a Datadog-compatible schema.
func DatadogSchema() core.Schema { return core.DatadogSchema() }

// CustomSchema returns a schema built from a custom function.
func CustomSchema(fn func(ev core.EventView) map[string]any) core.Schema {
	return core.CustomSchema(fn)
}
