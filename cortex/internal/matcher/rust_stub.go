//go:build !cortex_match
// +build !cortex_match

package matcher

// IsRustAvailable returns false when the cortex-match library is not linked.
func IsRustAvailable() bool {
	return false
}
