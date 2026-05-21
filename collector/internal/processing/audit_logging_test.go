package processing

import (
	"encoding/json"
	"sync"
	"testing"
)

// TestPIIAuditLogging verifies that PII redaction events are logged via the audit callback
func TestPIIAuditLogging(t *testing.T) {
	tests := []struct {
		name          string
		config        Config
		event         map[string]interface{}
		expectedLogs  []string // Expected logged field paths
		expectedCount int
	}{
		{
			name: "blocklist_redaction_audit",
			config: Config{
				Privacy: PrivacyConfig{
					Mode:      "warn",
					Blocklist: []string{"password", "api_key", "email"},
				},
			},
			event: map[string]interface{}{
				"username": "john_doe",
				"password": "secret123",
				"email":    "john@example.com",
				"api_key":  "sk-1234567890",
			},
			expectedLogs:  []string{"password", "email", "api_key"},
			expectedCount: 3,
		},
		{
			name: "nested_object_redaction_audit",
			config: Config{
				Privacy: PrivacyConfig{
					Mode:      "warn",
					Blocklist: []string{"password", "phone"},
				},
			},
			event: map[string]interface{}{
				"user": map[string]interface{}{
					"name":     "Alice",
					"password": "secret",
				},
				"contact": map[string]interface{}{
					"phone": "555-1234",
					"email": "alice@example.com",
				},
			},
			expectedLogs:  []string{"user.password", "contact.phone"},
			expectedCount: 2,
		},
		{
			name: "allowlist_redaction_audit",
			config: Config{
				Privacy: PrivacyConfig{
					Mode:      "warn",
					Allowlist: []string{"name", "email"},
				},
			},
			event: map[string]interface{}{
				"name":     "Bob",
				"email":    "bob@example.com",
				"password": "secret",
				"ssn":      "123-45-6789",
			},
			expectedLogs:  []string{"password", "ssn"},
			expectedCount: 2,
		},
		{
			name: "no_redaction_disabled_mode",
			config: Config{
				Privacy: PrivacyConfig{
					Mode:      "off",
					Blocklist: []string{"password"},
				},
			},
			event: map[string]interface{}{
				"password": "secret123",
			},
			expectedLogs:  []string{},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up audit log capture
			var (
				mu       sync.Mutex
				logCalls []map[string]interface{}
			)

			cfg := tt.config
			cfg.LogFunc = func(level string, message string, fields map[string]any) {
				mu.Lock()
				defer mu.Unlock()

				if message == "pii_redacted" {
					logCalls = append(logCalls, map[string]interface{}{
						"level":   level,
						"message": message,
						"fields":  fields,
					})
				}
			}

			// Create processor with audit logging
			p := &Processor{cfg: cfg}

			// Redact the event
			eventJSON, _ := json.Marshal(tt.event)
			redactedJSON, err := p.redactPII(eventJSON)
			if err != nil {
				t.Fatalf("redactPII failed: %v", err)
			}

			// Verify we got redacted output
			if redactedJSON == nil {
				t.Fatal("Expected redacted JSON, got nil")
			}

			// Verify audit logs
			mu.Lock()
			defer mu.Unlock()

			if len(logCalls) != tt.expectedCount {
				t.Errorf("Expected %d audit logs, got %d", tt.expectedCount, len(logCalls))
			}

			// Verify expected fields were logged
			loggedFields := make(map[string]bool)
			for _, call := range logCalls {
				fields := call["fields"].(map[string]interface{})
				if field, ok := fields["field"].(string); ok {
					loggedFields[field] = true
				}
			}

			for _, expectedField := range tt.expectedLogs {
				if !loggedFields[expectedField] {
					t.Errorf("Expected field %q to be logged, but wasn't. Logged: %v", expectedField, loggedFields)
				}
			}
		})
	}
}

// TestAuditLogFormat verifies the structure of audit log entries
func TestAuditLogFormat(t *testing.T) {
	var (
		mu       sync.Mutex
		logCalls []map[string]interface{}
	)

	cfg := Config{
		Privacy: PrivacyConfig{
			Mode:      "warn",
			Blocklist: []string{"password"},
		},
		LogFunc: func(level string, message string, fields map[string]any) {
			mu.Lock()
			defer mu.Unlock()

			if message == "pii_redacted" {
				logCalls = append(logCalls, map[string]interface{}{
					"level":   level,
					"message": message,
					"fields":  fields,
				})
			}
		},
	}

	event := map[string]interface{}{
		"username": "test_user",
		"password": "secret123",
	}

	eventJSON, _ := json.Marshal(event)
	p := &Processor{cfg: cfg}

	// Redact event
	p.redactPII(eventJSON)

	// Verify audit log format
	mu.Lock()
	defer mu.Unlock()

	if len(logCalls) == 0 {
		t.Fatal("No audit logs were generated")
	}

	call := logCalls[0]

	// Check level
	if level, ok := call["level"].(string); !ok || level != "info" {
		t.Errorf("Expected level 'info', got %q", level)
	}

	// Check message
	if message, ok := call["message"].(string); !ok || message != "pii_redacted" {
		t.Errorf("Expected message 'pii_redacted', got %q", message)
	}

	// Check fields structure
	fields, ok := call["fields"].(map[string]interface{})
	if !ok {
		t.Fatal("Fields is not a map")
	}

	// Verify field path is present
	if _, ok := fields["field"]; !ok {
		t.Error("Missing 'field' in audit log fields")
	}
}

// TestAuditLoggingMultipleRedactions verifies multiple redactions are all logged
func TestAuditLoggingMultipleRedactions(t *testing.T) {
	var (
		mu       sync.Mutex
		logCalls []string
	)

	cfg := Config{
		Privacy: PrivacyConfig{
			Mode:      "warn",
			Blocklist: []string{"password", "token", "api_key", "secret"},
		},
		LogFunc: func(level string, message string, fields map[string]any) {
			mu.Lock()
			defer mu.Unlock()

			if message == "pii_redacted" {
				if field, ok := fields["field"].(string); ok {
					logCalls = append(logCalls, field)
				}
			}
		},
	}

	event := map[string]interface{}{
		"username": "test_user",
		"password": "secret123",
		"token":    "abc123xyz",
		"api_key":  "sk-12345",
		"secret":   "hidden",
		"public":   "visible",
	}

	eventJSON, _ := json.Marshal(event)
	p := &Processor{cfg: cfg}
	p.redactPII(eventJSON)

	// Verify all sensitive fields were logged
	mu.Lock()
	defer mu.Unlock()

	expectedFields := map[string]bool{
		"password": false,
		"token":    false,
		"api_key":  false,
		"secret":   false,
	}

	for _, logged := range logCalls {
		if _, exists := expectedFields[logged]; exists {
			expectedFields[logged] = true
		}
	}

	for field, found := range expectedFields {
		if !found {
			t.Errorf("Expected field %q to be audited, but wasn't. Logged: %v", field, logCalls)
		}
	}

	if len(logCalls) != 4 {
		t.Errorf("Expected 4 audit logs, got %d: %v", len(logCalls), logCalls)
	}
}

// TestAuditLoggingWithNestedStructures verifies nested object redactions are properly logged
func TestAuditLoggingWithNestedStructures(t *testing.T) {
	var (
		mu       sync.Mutex
		logCalls []string
	)

	cfg := Config{
		Privacy: PrivacyConfig{
			Mode:      "warn",
			Blocklist: []string{"password", "ssn", "phone"},
		},
		LogFunc: func(level string, message string, fields map[string]any) {
			mu.Lock()
			defer mu.Unlock()

			if message == "pii_redacted" {
				if field, ok := fields["field"].(string); ok {
					logCalls = append(logCalls, field)
				}
			}
		},
	}

	event := map[string]interface{}{
		"user": map[string]interface{}{
			"name":     "Alice",
			"password": "secret",
			"profile": map[string]interface{}{
				"ssn":   "123-45-6789",
				"phone": "555-1234",
			},
		},
		"settings": map[string]interface{}{
			"password": "new_secret",
		},
	}

	eventJSON, _ := json.Marshal(event)
	p := &Processor{cfg: cfg}
	p.redactPII(eventJSON)

	// Verify nested paths are logged correctly
	mu.Lock()
	defer mu.Unlock()

	expectedPaths := map[string]bool{
		"user.password":            false,
		"user.profile.ssn":         false,
		"user.profile.phone":       false,
		"settings.password":        false,
	}

	for _, logged := range logCalls {
		if _, exists := expectedPaths[logged]; exists {
			expectedPaths[logged] = true
		}
	}

	for path, found := range expectedPaths {
		if !found {
			t.Errorf("Expected path %q to be audited, but wasn't. Logged: %v", path, logCalls)
		}
	}
}
