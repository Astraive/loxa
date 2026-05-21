package redaction

import (
	"github.com/astraive/loxa-go/src/core"
)

// DefaultRedactor returns a Redactor that replaces common sensitive keys.
func DefaultRedactor() core.Redactor {
	return core.DefaultRedactor()
}

// RedactKeys returns a Redactor that replaces values for the given keys.
func RedactKeys(keys ...string) core.Redactor {
	return core.RedactKeys(keys...)
}

// HashKeys returns a Redactor that hashes values for the given keys.
func HashKeys(keys ...string) core.Redactor {
	return core.HashKeys(keys...)
}

// MaskKeys returns a Redactor that partially masks values for given keys.
func MaskKeys(keys ...string) core.Redactor {
	return core.MaskKeys(keys...)
}

// DropKeys returns a Redactor that removes fields with the given keys.
func DropKeys(keys ...string) core.Redactor {
	return core.DropKeys(keys...)
}

// ComposeRedactors combines multiple redactors; the first match wins.
func ComposeRedactors(redactors ...core.Redactor) core.Redactor {
	return core.ComposeRedactors(redactors...)
}
