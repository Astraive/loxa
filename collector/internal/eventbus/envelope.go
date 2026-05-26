package eventbus

import "time"

// Envelope is the canonical event wrapper that flows through the event bus.
// Body contains the validated/redacted event JSON from collector ingest.
type Envelope struct {
	ID        string            `json:"id"`
	TenantID  string            `json:"tenant_id,omitempty"`
	Service   string            `json:"service,omitempty"`
	Event     string            `json:"event"`
	Timestamp time.Time         `json:"timestamp"`
	Headers   map[string]string `json:"headers,omitempty"`
	Body      []byte            `json:"body"`
	Attempts  int               `json:"attempts,omitempty"`
}
