package main

import (
	"net/http/httptest"
	"testing"
	"time"

	serverruntime "github.com/astraive/loza/collector/internal/server"
)

func TestParseTailFilters(t *testing.T) {
	req := httptest.NewRequest("GET", "/tail?since=2026-01-01T00:00:00Z&after_event_id=evt-1&service=checkout&kind=log&trace_id=tr-1&incident_id=inc-1&limit=25", nil)
	filters, err := serverruntime.ParseTailFilters(req)
	if err != nil {
		t.Fatalf("parse filters: %v", err)
	}
	if !filters.Since.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected since filter: %+v", filters)
	}
	if filters.AfterEventID != "evt-1" || filters.Service != "checkout" || filters.Kind != "log" || filters.TraceID != "tr-1" || filters.IncidentID != "inc-1" || filters.Limit != 25 {
		t.Fatalf("unexpected filters: %+v", filters)
	}
}

func TestRawMatchesTailFilters(t *testing.T) {
	raw := []byte(`{"id":"evt-1","service":"checkout","kind":"log","trace_id":"tr-1","incident_id":"inc-1"}`)
	if !rawMatchesTailFilters(raw, serverruntime.TailFilters{Service: "checkout", Kind: "log", TraceID: "tr-1", IncidentID: "inc-1"}) {
		t.Fatal("expected raw payload to match filters")
	}
	if rawMatchesTailFilters(raw, serverruntime.TailFilters{Service: "billing"}) {
		t.Fatal("expected service mismatch to fail")
	}
}
