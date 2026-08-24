package utils_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/astraive/loza/sdks/go/src/utils"
)

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestParseNDJSONSkipsBlankLinesAndReportsMalformedLine(t *testing.T) {
	events, err := utils.ParseNDJSON([]byte("\n {\"event\":\"ok\"} \n\n"))
	if err != nil {
		t.Fatalf("parse blank lines: %v", err)
	}
	if len(events) != 1 || events[0]["event"] != "ok" {
		t.Fatalf("events = %#v", events)
	}
	_, err = utils.ParseNDJSON([]byte("{\"ok\":true}\nnot-json\n"))
	if err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("malformed error = %v", err)
	}
}

func TestParseNDJSONReaderReportsReadError(t *testing.T) {
	if _, err := utils.ParseNDJSONReader(failingReader{}); err == nil || !strings.Contains(err.Error(), "read ndjson") {
		t.Fatalf("read error = %v", err)
	}
}
