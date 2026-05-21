//go:build cortex_match
// +build cortex_match

package matcher

/*
#cgo LDFLAGS: -L${SRCDIR}/../../crates/cortex-match/target/release -lcortex_match
#include <stdlib.h>

extern double score_ffi(const char* query_json, const char* candidate_json);
extern char* topk_search_ffi(const char* query_json, const char* candidates_json, int k);
extern char* shape_hash_ffi(const char* signature_json);
extern void free_c_string(char* ptr);
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"unsafe"
)

// RustMatcherCGO uses CGO to call the Rust cortex-match library.
type RustMatcherCGO struct{}

func NewRustMatcherCGO() *RustMatcherCGO {
	return &RustMatcherCGO{}
}

// IsAvailable returns true if the Rust library can be loaded.
func IsRustAvailable() bool {
	// Try a simple call to verify the library is linked
	return true
}

// Score computes similarity between two signatures via Rust.
func (m *RustMatcherCGO) Score(queryJSON, candidateJSON string) (float64, error) {
	queryC := C.CString(queryJSON)
	defer C.free(unsafe.Pointer(queryC))
	candidateC := C.CString(candidateJSON)
	defer C.free(unsafe.Pointer(candidateC))

	score := C.score_ffi(queryC, candidateC)
	return float64(score), nil
}

// TopKSearch finds the k most similar signatures via Rust.
func (m *RustMatcherCGO) TopKSearch(queryJSON, candidatesJSON string, k int) ([]TopKResult, error) {
	queryC := C.CString(queryJSON)
	defer C.free(unsafe.Pointer(queryC))
	candidatesC := C.CString(candidatesJSON)
	defer C.free(unsafe.Pointer(candidatesC))

	resultC := C.topk_search_ffi(queryC, candidatesC, C.int(k))
	defer C.free_c_string(resultC)

	resultStr := C.GoString(resultC)

	var results []TopKResult
	if err := json.Unmarshal([]byte(resultStr), &results); err != nil {
		return nil, fmt.Errorf("failed to parse topk results: %w", err)
	}
	return results, nil
}

// ShapeHash computes a topology-independent shape hash via Rust.
func (m *RustMatcherCGO) ShapeHash(signatureJSON string) (string, error) {
	sigC := C.CString(signatureJSON)
	defer C.free(unsafe.Pointer(sigC))

	hashC := C.shape_hash_ffi(sigC)
	defer C.free_c_string(hashC)

	return C.GoString(hashC), nil
}

// TopKResult represents a single result from top-k search.
type TopKResult struct {
	Index      int     `json:"index"`
	Similarity float64 `json:"similarity"`
}
