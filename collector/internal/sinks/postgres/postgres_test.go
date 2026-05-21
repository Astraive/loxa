package postgres

import (
	"context"
	"strings"
	"testing"
)

func TestNewRejectsInvalidTableIdentifier(t *testing.T) {
	_, err := New(context.Background(), Config{
		DSN:   "postgres://user:pass@localhost:5432/test?sslmode=disable",
		Table: "public.events;DROP TABLE x",
	})
	if err == nil {
		t.Fatalf("expected invalid table identifier error")
	}
	if !strings.Contains(err.Error(), "invalid table identifier") {
		t.Fatalf("expected invalid identifier error, got: %v", err)
	}
}

func TestNewRejectsInvalidSchemaPath(t *testing.T) {
	_, err := New(context.Background(), Config{
		DSN:   "postgres://user:pass@localhost:5432/test?sslmode=disable",
		Table: "public.events",
		Schema: map[string]string{
			"status_code": "http..status",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid schema path") {
		t.Fatalf("expected invalid schema path error, got: %v", err)
	}
}

func TestNewRejectsRawColumnConflict(t *testing.T) {
	_, err := New(context.Background(), Config{
		DSN:       "postgres://user:pass@localhost:5432/test?sslmode=disable",
		Table:     "public.events",
		RawColumn: "raw",
		StoreRaw:  true,
		Schema: map[string]string{
			"raw": "event_id",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "conflicts with schema column") {
		t.Fatalf("expected raw-column conflict error, got: %v", err)
	}
}
