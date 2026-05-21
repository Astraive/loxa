// Package conformance contains reusable sink behavior checks.
//
// Every production sink should wire this suite into its package tests. The
// suite intentionally starts with the collector's current event.Sink interface
// and can be extended as the full health, batch, idempotency, timeout,
// migration, and partial-failure sink contract lands.
package conformance
