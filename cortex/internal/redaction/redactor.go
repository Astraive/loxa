package redaction

import (
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

var (
	emailRegex  = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	ipRegex     = regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)
	ccRegex     = regexp.MustCompile(`\b\d{4}[\- ]?\d{4}[\- ]?\d{4}[\- ]?\d{4}\b`)
	ssnRegex    = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	phoneRegex  = regexp.MustCompile(`\b\+?1?[\- ]?\(?\d{3}\)?[\- ]?\d{3}[\- ]?\d{4}\b`)
)

// Mode controls redaction behavior.
type Mode string

const (
	ModeEnforce Mode = "enforce"
	ModeLog     Mode = "log"
)

// Redactor detects and optionally redacts PII from event data.
type Redactor struct {
	mode    Mode
	patterns []*regexp.Regexp
}

// New creates a PII redactor with the given mode.
func New(mode Mode) *Redactor {
	return &Redactor{
		mode: mode,
		patterns: []*regexp.Regexp{
			emailRegex,
			ccRegex,
			ssnRegex,
			// IP and phone are less sensitive, include only in enforce mode
		},
	}
}

// NewStrict creates a PII redactor that catches all patterns including IPs and phone numbers.
func NewStrict(mode Mode) *Redactor {
	return &Redactor{
		mode: mode,
		patterns: []*regexp.Regexp{
			emailRegex,
			ipRegex,
			ccRegex,
			ssnRegex,
			phoneRegex,
		},
	}
}

// RedactMap walks a map and redacts PII in string values.
func (r *Redactor) RedactMap(data map[string]interface{}) map[string]interface{} {
	if r == nil || r.mode == "" {
		return data
	}
	return r.walkMap(data)
}

func (r *Redactor) walkMap(data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(data))
	for k, v := range data {
		switch val := v.(type) {
		case string:
			result[k] = r.redactString(k, val)
		case map[string]interface{}:
			result[k] = r.walkMap(val)
		case []interface{}:
			result[k] = r.walkSlice(val)
		default:
			result[k] = v
		}
	}
	return result
}

func (r *Redactor) walkSlice(data []interface{}) []interface{} {
	result := make([]interface{}, len(data))
	for i, v := range data {
		switch val := v.(type) {
		case string:
			result[i] = r.redactString("", val)
		case map[string]interface{}:
			result[i] = r.walkMap(val)
		case []interface{}:
			result[i] = r.walkSlice(val)
		default:
			result[i] = v
		}
	}
	return result
}

func (r *Redactor) redactString(key, value string) string {
	// Check well-known sensitive keys first
	lowerKey := strings.ToLower(key)
	sensitiveKeys := []string{"password", "secret", "token", "api_key", "apikey", "authorization", "credit_card", "ssn", "email"}
	for _, sk := range sensitiveKeys {
		if strings.Contains(lowerKey, sk) {
			if r.mode == ModeEnforce {
				return "[REDACTED]"
			}
			log.Warn().Str("key", key).Msg("PII detected in sensitive field")
			return value
		}
	}

	// Check patterns
	for _, pattern := range r.patterns {
		if pattern.MatchString(value) {
			if r.mode == ModeEnforce {
				return pattern.ReplaceAllString(value, "[REDACTED]")
			}
			log.Warn().Str("key", key).Str("pattern", pattern.String()).Msg("PII pattern detected")
		}
	}

	return value
}
