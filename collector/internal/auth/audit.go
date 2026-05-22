package auth

import (
	"context"
	"log/slog"
	"time"
)

// AuditEvent represents an auth-related audit event.
type AuditEvent struct {
	Timestamp  time.Time
	Event      string // key.authenticated, key.failed, key.rate_limited, key.permission_denied, etc.
	KeyID      string
	KeyKind    KeyKind
	OrgID      string
	ProjectID  string
	SourceIP   string
	UserAgent  string
	Endpoint   string
	Permission string
	Reason     string
}

// AuditLogger emits audit events for auth-related actions.
type AuditLogger interface {
	LogAudit(ctx context.Context, event AuditEvent)
}

// SlogAuditLogger implements AuditLogger using slog.
type SlogAuditLogger struct {
	logger *slog.Logger
}

// NewSlogAuditLogger creates an audit logger backed by slog.
func NewSlogAuditLogger(logger *slog.Logger) *SlogAuditLogger {
	if logger == nil {
		logger = slog.Default()
	}
	return &SlogAuditLogger{logger: logger}
}

// LogAudit emits an audit event as a structured log.
func (l *SlogAuditLogger) LogAudit(ctx context.Context, event AuditEvent) {
	attrs := []slog.Attr{
		slog.String("audit_event", event.Event),
		slog.String("key_id", event.KeyID),
		slog.String("key_kind", string(event.KeyKind)),
		slog.String("org_id", event.OrgID),
		slog.String("project_id", event.ProjectID),
		slog.String("source_ip", event.SourceIP),
		slog.String("user_agent", event.UserAgent),
		slog.String("endpoint", event.Endpoint),
	}
	if event.Permission != "" {
		attrs = append(attrs, slog.String("permission", event.Permission))
	}
	if event.Reason != "" {
		attrs = append(attrs, slog.String("reason", event.Reason))
	}

	l.logger.LogAttrs(ctx, slog.LevelInfo, "audit", attrs...)
}

// Audit event names.
const (
	AuditKeyAuthenticated  = "key.authenticated"
	AuditKeyFailed         = "key.failed"
	AuditKeyRateLimited    = "key.rate_limited"
	AuditKeyPermissionDenied = "key.permission_denied"
	AuditKeyEnvDenied      = "key.env_denied"
	AuditKeyOriginDenied   = "key.origin_denied"
	AuditKeyIPDenied       = "key.ip_denied"
	AuditKeyPayloadTooLarge = "key.payload_too_large"
)
