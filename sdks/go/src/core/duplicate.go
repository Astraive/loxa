package core

import speccontract "github.com/astraive/loxa/spec/generated/go/contract"

// CanonicalFieldSet is the generated set of top-level canonical JSON field names.
var CanonicalFieldSet = speccontract.CanonicalFieldSet

// IsCanonical returns true if key matches a canonical field name.
func IsCanonical(key string) bool {
	return speccontract.IsCanonical(key)
}
