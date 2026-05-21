package processing

import (
	"encoding/json"
	"testing"
)

func TestRedactPIIBlocklist(t *testing.T) {
	tests := []struct {
		name      string
		input     map[string]interface{}
		blocklist []string
		expected  map[string]interface{}
	}{
		{
			name: "redact email field",
			input: map[string]interface{}{
				"user": map[string]interface{}{
					"name":  "Alice",
					"email": "alice@example.com",
				},
			},
			blocklist: []string{"email"},
			expected: map[string]interface{}{
				"user": map[string]interface{}{
					"name":  "Alice",
					"email": "[REDACTED]",
				},
			},
		},
		{
			name: "redact password field",
			input: map[string]interface{}{
				"auth": map[string]interface{}{
					"username": "alice",
					"password": "secret123",
				},
			},
			blocklist: []string{"password"},
			expected: map[string]interface{}{
				"auth": map[string]interface{}{
					"username": "alice",
					"password": "[REDACTED]",
				},
			},
		},
		{
			name: "redact multiple fields",
			input: map[string]interface{}{
				"user": map[string]interface{}{
					"name":   "Alice",
					"email":  "alice@example.com",
					"ssn":    "123-45-6789",
					"phone":  "555-1234",
				},
			},
			blocklist: []string{"email", "ssn", "phone"},
			expected: map[string]interface{}{
				"user": map[string]interface{}{
					"name":   "Alice",
					"email":  "[REDACTED]",
					"ssn":    "[REDACTED]",
					"phone":  "[REDACTED]",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Processor{
				cfg: Config{
					Privacy: PrivacyConfig{
						Mode:      "enforce",
						Blocklist: tt.blocklist,
					},
				},
			}

			inputJSON, _ := json.Marshal(tt.input)
			redacted, err := p.redactPII(inputJSON)
			if err != nil {
				t.Fatalf("redactPII failed: %v", err)
			}

			var result map[string]interface{}
			if err := json.Unmarshal(redacted, &result); err != nil {
				t.Fatalf("unmarshal result failed: %v", err)
			}

			// Compare nested structures
			if !structsEqual(result, tt.expected) {
				resultJSON, _ := json.MarshalIndent(result, "", "  ")
				expectedJSON, _ := json.MarshalIndent(tt.expected, "", "  ")
				t.Errorf("redaction mismatch\nGot:\n%s\nExpected:\n%s", string(resultJSON), string(expectedJSON))
			}
		})
	}
}

func TestRedactPIIAllowlist(t *testing.T) {
	p := &Processor{
		cfg: Config{
			Privacy: PrivacyConfig{
				Mode:      "enforce",
				Allowlist: []string{"name", "age"},
			},
		},
	}

	input := map[string]interface{}{
		"name":   "Alice",
		"age":    30,
		"email":  "alice@example.com",
		"ssn":    "123-45-6789",
	}

	inputJSON, _ := json.Marshal(input)
	redacted, err := p.redactPII(inputJSON)
	if err != nil {
		t.Fatalf("redactPII failed: %v", err)
	}

	var result map[string]interface{}
	json.Unmarshal(redacted, &result)

	// name and age should be unchanged
	if result["name"] != "Alice" {
		t.Errorf("name should not be redacted, got %v", result["name"])
	}
	if result["age"] != float64(30) { // JSON unmarshals numbers as float64
		t.Errorf("age should not be redacted, got %v", result["age"])
	}

	// email and ssn should be redacted
	if result["email"] != "[REDACTED]" {
		t.Errorf("email should be redacted, got %v", result["email"])
	}
	if result["ssn"] != "[REDACTED]" {
		t.Errorf("ssn should be redacted, got %v", result["ssn"])
	}
}

func TestRedactPIIDisabled(t *testing.T) {
	p := &Processor{
		cfg: Config{
			Privacy: PrivacyConfig{
				Mode: "off",
			},
		},
	}

	input := map[string]interface{}{
		"email":    "alice@example.com",
		"password": "secret",
	}

	inputJSON, _ := json.Marshal(input)
	redacted, err := p.redactPII(inputJSON)
	if err != nil {
		t.Fatalf("redactPII failed: %v", err)
	}

	// Should be unchanged
	if string(redacted) != string(inputJSON) {
		t.Errorf("redaction should be disabled, but data was modified")
	}
}

func TestRedactNestedArrays(t *testing.T) {
	p := &Processor{
		cfg: Config{
			Privacy: PrivacyConfig{
				Mode:      "enforce",
				Blocklist: []string{"password"},
			},
		},
	}

	input := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{
				"name":     "Alice",
				"password": "secret1",
			},
			map[string]interface{}{
				"name":     "Bob",
				"password": "secret2",
			},
		},
	}

	inputJSON, _ := json.Marshal(input)
	redacted, err := p.redactPII(inputJSON)
	if err != nil {
		t.Fatalf("redactPII failed: %v", err)
	}

	var result map[string]interface{}
	json.Unmarshal(redacted, &result)

	users := result["users"].([]interface{})
	alice := users[0].(map[string]interface{})
	bob := users[1].(map[string]interface{})

	if alice["password"] != "[REDACTED]" {
		t.Errorf("Alice's password should be redacted, got %v", alice["password"])
	}
	if bob["password"] != "[REDACTED]" {
		t.Errorf("Bob's password should be redacted, got %v", bob["password"])
	}
}

func TestMatchesPattern(t *testing.T) {
	tests := []struct {
		fieldPath string
		keyLower  string
		pattern   string
		expected  bool
	}{
		{"email", "email", "email", true},
		{"user.email", "email", "email", true},
		{"auth.user.email", "email", "email", true},
		{"email", "email", "password", false},
		{"user.email", "email", "user.*", true},
		{"user.name", "name", "user.*", true},
		{"admin.email", "email", "user.*", false},
	}

	p := &Processor{}
	for _, tt := range tests {
		result := p.matchesPattern(tt.fieldPath, tt.keyLower, tt.pattern)
		if result != tt.expected {
			t.Errorf("matchesPattern(%q, %q, %q) = %v, want %v",
				tt.fieldPath, tt.keyLower, tt.pattern, result, tt.expected)
		}
	}
}

// Helper function to compare nested structures
func structsEqual(a, b interface{}) bool {
	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)
	return string(aJSON) == string(bJSON)
}
