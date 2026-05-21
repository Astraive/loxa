package core

import "time"

// Params holds all inputs for starting a canonical event.
// Canonical fields are first-class struct members; extra business context
// goes in Custom or is added later via Enrich.
type Params struct {
	// ── Identity ────────────────────────────────────────────────────────────
	Event   string
	Name    string
	Kind    string
	Message string
	Level   Level

	// ── Correlation IDs ──────────────────────────────────────────────────────
	RequestID string
	TraceID   string
	SpanID    string
	ParentID  string

	// ── Service metadata ─────────────────────────────────────────────────────
	Service      string
	Version      string
	Environment  string
	DeploymentID string
	Region       string
	Host         string
	Runtime      string

	// ── Request metadata ─────────────────────────────────────────────────────
	Method     string
	Path       string
	Route      string
	StatusCode int
	DurationMS int64
	Outcome    string

	// ── Canonical subject identifiers ───────────────────────────────────────
	UserID         string
	TenantID       string
	WorkspaceID    string
	OrganizationID string
	SessionID      string

	// ── Timing ───────────────────────────────────────────────────────────────
	StartedAt time.Time

	// ── Custom business context ───────────────────────────────────────────────
	// Custom holds domain-specific attrs that LOXA-Go does not know in advance.
	// These are copied into Event.Attrs during StartEvent.
	Custom []Attr
}

// With returns a copy of p with additional attrs appended to Custom.
// It enables a builder style:
//
//	loxa.Params{Event: "checkout.request"}.With(
//	    loxa.String("tenant.id", tenantID),
//	)
func (p Params) With(attrs ...Attr) Params {
	p.Custom = append(p.Custom, attrs...)
	return p
}
