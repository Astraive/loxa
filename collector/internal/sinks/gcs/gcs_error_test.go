package gcs

import (
	"errors"
	"testing"
)

func TestJoinWriteCloseErrorIncludesBothCauses(t *testing.T) {
	writeErr := errors.New("write failed")
	closeErr := errors.New("close failed")

	err := joinWriteCloseError("events.ndjson", writeErr, closeErr)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, writeErr) {
		t.Fatalf("expected joined error to include write error")
	}
	if !errors.Is(err, closeErr) {
		t.Fatalf("expected joined error to include close error")
	}
}

func TestJoinWriteCloseErrorPreservesWriteErrorWhenCloseSucceeds(t *testing.T) {
	writeErr := errors.New("write failed")

	err := joinWriteCloseError("events.ndjson", writeErr, nil)
	if !errors.Is(err, writeErr) {
		t.Fatalf("expected wrapped write error, got %v", err)
	}
}
