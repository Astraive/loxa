package utils_test

import (
	"testing"

	"github.com/astraive/loxa/sdks/go/src/utils"
)

func TestParseNDJSON(t *testing.T) {
	in := []byte("{\"event\":\"a\"}\n{\"event\":\"b\",\"status\":200}\n")
	events, err := utils.ParseNDJSON(in)
	if err != nil {
		t.Fatalf("parse ndjson: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0]["event"] != "a" || events[1]["event"] != "b" {
		t.Fatalf("unexpected parse result: %#v", events)
	}
}
