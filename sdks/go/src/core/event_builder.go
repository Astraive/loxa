package core

import (
	"os"
	"runtime"
)

// buildEvent constructs a new Event from params and cfg.
// This lives in the root package to avoid circular imports.
// Requirements: 1.2
func buildEvent(params Params, cfg *Config) *Event {
	now := cfg.Clock.Now()
	if params.StartedAt.IsZero() {
		params.StartedAt = now
	}

	ev := &Event{}
	ev.state = EventStateCreated // Start in created state per requirement 1.2
	ev.Timestamp = now
	ev.StartedAt = params.StartedAt
	ev.SchemaVersion = LOZA_SPEC_VERSION
	ev.EventVersion = LOZA_EVENT_VERSION

	// IDs
	ev.EventID = cfg.IDGen.NewID()
	if params.RequestID != "" {
		ev.RequestID = params.RequestID
	} else {
		ev.RequestID = cfg.IDGen.NewID()
	}
	
	// Trace context propagation (Requirements: 39.3, 39.4, 39.5, 39.6)
	// Store caller-provided values; defer generation to ensureTraceContext()
	// which runs after sampling, so sampled-out events skip the PRNG cost.
	ev.TraceID = params.TraceID
	ev.SpanID = params.SpanID
	
	// Parent span ID is optional and only set if provided (Requirement 39.5)
	ev.ParentID = params.ParentID
	ev.IncidentID = params.IncidentID

	// Classification
	ev.Level = params.Level
	if ev.Level == 0 {
		ev.Level = LevelInfo
	}
	ev.Event = firstNonEmpty(params.Event, params.Name)
	if ev.Event == "" {
		ev.Event = "event"
	}
	ev.Kind = firstNonEmpty(params.Kind, "event")
	ev.Message = params.Message
	ev.Outcome = params.Outcome

	// Service metadata: params override cfg defaults
	ev.Service = firstNonEmpty(params.Service, cfg.Service)
	ev.Version = firstNonEmpty(params.Version, cfg.Version)
	ev.Environment = firstNonEmpty(params.Environment, cfg.Environment)
	ev.DeploymentID = firstNonEmpty(params.DeploymentID, cfg.DeploymentID)
	ev.Region = firstNonEmpty(params.Region, cfg.Region)
	ev.Host = params.Host
	ev.Runtime = params.Runtime

	if cfg.IncludeHost && ev.Host == "" {
		if h, err := os.Hostname(); err == nil {
			ev.Host = h
		}
	}
	if cfg.IncludeRuntime && ev.Runtime == "" {
		ev.Runtime = runtime.Version()
	}

	// Request metadata
	ev.Method = params.Method
	ev.Path = params.Path
	ev.Route = params.Route
	ev.StatusCode = params.StatusCode
	ev.DurationMS = params.DurationMS

	// Canonical subject identifiers are included as attrs.
	if params.UserID != "" {
		_ = ev.AddAttrs([]Attr{UserID(params.UserID)})
	}
	if params.TenantID != "" {
		_ = ev.AddAttrs([]Attr{TenantID(params.TenantID)})
	}
	if params.WorkspaceID != "" {
		_ = ev.AddAttrs([]Attr{WorkspaceID(params.WorkspaceID)})
	}
	if params.OrganizationID != "" {
		_ = ev.AddAttrs([]Attr{OrganizationID(params.OrganizationID)})
	}
	if params.SessionID != "" {
		_ = ev.AddAttrs([]Attr{SessionID(params.SessionID)})
	}
	if cfg.Alias != "" {
		_ = ev.AddAttrs([]Attr{String("loza.alias", cfg.Alias)})
	}
	// Copy custom params into Attrs
	if len(params.Custom) > 0 {
		attrs := make([]Attr, len(params.Custom))
		copy(attrs, params.Custom)
		_ = ev.AddAttrs(attrs)
	}

	return ev
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
