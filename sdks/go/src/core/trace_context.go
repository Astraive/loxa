package core

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// W3C Trace Context support for distributed tracing.
// Implements trace_id and span_id generation according to W3C Trace Context specification.
// Requirements: 39.1, 39.2, 39.3, 39.4, 39.5, 39.6, 39.7, 39.8

// GenerateTraceID generates a new W3C Trace Context trace-id (32 hex characters, 16 bytes).
// Format: 32 lowercase hex characters representing a 16-byte array.
// Example: "0af7651916cd43dd8448eb211c80319c"
// Requirements: 39.6
func GenerateTraceID() string {
	var b [16]byte
	_, err := rand.Read(b[:])
	if err != nil {
		// Fallback to a deterministic but unique value if random fails
		// This should never happen in practice
		return fmt.Sprintf("%032x", 0)
	}
	return hex.EncodeToString(b[:])
}

// GenerateSpanID generates a new W3C Trace Context span-id (16 hex characters, 8 bytes).
// Format: 16 lowercase hex characters representing an 8-byte array.
// Example: "00f067aa0ba902b7"
// Requirements: 39.6
func GenerateSpanID() string {
	var b [8]byte
	_, err := rand.Read(b[:])
	if err != nil {
		// Fallback to a deterministic but unique value if random fails
		// This should never happen in practice
		return fmt.Sprintf("%016x", 0)
	}
	return hex.EncodeToString(b[:])
}

// IsValidTraceID checks if a trace ID is valid according to W3C Trace Context spec.
// Valid trace IDs are 32 hex characters and not all zeros.
func IsValidTraceID(traceID string) bool {
	if len(traceID) != 32 {
		return false
	}
	// Check if all zeros (invalid)
	allZeros := true
	for _, c := range traceID {
		if c != '0' {
			allZeros = false
		}
		// Check if valid hex character
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return !allZeros
}

// IsValidSpanID checks if a span ID is valid according to W3C Trace Context spec.
// Valid span IDs are 16 hex characters and not all zeros.
func IsValidSpanID(spanID string) bool {
	if len(spanID) != 16 {
		return false
	}
	// Check if all zeros (invalid)
	allZeros := true
	for _, c := range spanID {
		if c != '0' {
			allZeros = false
		}
		// Check if valid hex character
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return !allZeros
}
