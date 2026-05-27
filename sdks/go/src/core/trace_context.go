package core

import (
	cr "crypto/rand"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
	"sync"
)

// W3C Trace Context support for distributed tracing.
// Implements trace_id and span_id generation according to W3C Trace Context specification.
// Requirements: 39.1, 39.2, 39.3, 39.4, 39.5, 39.6, 39.7, 39.8

// traceRand is a fast PRNG for trace/span ID generation.
// These IDs are unique identifiers, not cryptographic secrets, so a fast PRNG is appropriate.
// The PRNG is seeded from crypto/rand at init time for uniqueness across processes.
var (
	traceRand   *rand.Rand
	traceRandMu sync.Mutex
)

func init() {
	// Seed from crypto/rand once at init for cross-process uniqueness.
	// After init, all ID generation uses the fast PRNG (~10ns vs ~300ns for crypto/rand).
	var seed [32]byte
	if _, err := cr.Read(seed[:]); err == nil {
		traceRand = rand.New(rand.NewChaCha8(seed))
	}
}

// TraceContext holds trace_id and span_id together for single-generation.
type TraceContext struct {
	TraceID string
	SpanID  string
}

// GenerateTraceContext generates both trace_id and span_id in a single PRNG call.
// This reduces 2 syscalls to 1 lock+generate operation.
func GenerateTraceContext() TraceContext {
	var buf [24]byte // 16 bytes trace + 8 bytes span
	traceRandMu.Lock()
	if traceRand != nil {
		for i := range buf {
			buf[i] = byte(traceRand.Uint32())
		}
		traceRandMu.Unlock()
	} else {
		traceRandMu.Unlock()
		if _, err := cr.Read(buf[:]); err != nil {
			return TraceContext{
				TraceID: fmt.Sprintf("%032x", 0),
				SpanID:  fmt.Sprintf("%016x", 0),
			}
		}
	}
	return TraceContext{
		TraceID: hex.EncodeToString(buf[:16]),
		SpanID:  hex.EncodeToString(buf[16:24]),
	}
}

// GenerateTraceID generates a new W3C Trace Context trace-id (32 hex characters, 16 bytes).
// Format: 32 lowercase hex characters representing a 16-byte array.
// Example: "0af7651916cd43dd8448eb211c80319c"
// Requirements: 39.6
func GenerateTraceID() string {
	var b [16]byte
	traceRandMu.Lock()
	if traceRand != nil {
		for i := range b {
			b[i] = byte(traceRand.Uint32())
		}
		traceRandMu.Unlock()
	} else {
		traceRandMu.Unlock()
		if _, err := cr.Read(b[:]); err != nil {
			return fmt.Sprintf("%032x", 0)
		}
	}
	return hex.EncodeToString(b[:])
}

// GenerateSpanID generates a new W3C Trace Context span-id (16 hex characters, 8 bytes).
// Format: 16 lowercase hex characters representing an 8-byte array.
// Example: "00f067aa0ba902b7"
// Requirements: 39.6
func GenerateSpanID() string {
	var b [8]byte
	traceRandMu.Lock()
	if traceRand != nil {
		for i := range b {
			b[i] = byte(traceRand.Uint32())
		}
		traceRandMu.Unlock()
	} else {
		traceRandMu.Unlock()
		if _, err := cr.Read(b[:]); err != nil {
			return fmt.Sprintf("%016x", 0)
		}
	}
	return hex.EncodeToString(b[:])
}

// IsValidTraceID checks if a trace ID is valid according to W3C Trace Context spec.
// Valid trace IDs are 32 hex characters and not all zeros.
func IsValidTraceID(traceID string) bool {
	if len(traceID) != 32 {
		return false
	}
	allZeros := true
	for _, c := range traceID {
		if c != '0' {
			allZeros = false
		}
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
	allZeros := true
	for _, c := range spanID {
		if c != '0' {
			allZeros = false
		}
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return !allZeros
}
