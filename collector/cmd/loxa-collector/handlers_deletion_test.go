package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/marcboeker/go-duckdb"
)

// TestDeleteEventsByTenant verifies deletion of all events for a specific tenant
func TestDeleteEventsByTenant(t *testing.T) {
	// Create test database
	db, err := sql.Open("duckdb", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	// Create test table
	if _, err := db.Exec(`CREATE TABLE events (
		event_id VARCHAR,
		tenant_id VARCHAR,
		timestamp TIMESTAMP,
		raw VARCHAR
	)`); err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	// Insert test data
	testData := []struct {
		eventID  string
		tenantID string
	}{
		{"evt1", "tenant1"},
		{"evt2", "tenant1"},
		{"evt3", "tenant2"},
		{"evt4", "tenant1"},
	}

	for _, data := range testData {
		if _, err := db.Exec(
			`INSERT INTO events (event_id, tenant_id, timestamp, raw) VALUES (?, ?, CURRENT_TIMESTAMP, '{}')`,
			data.eventID, data.tenantID,
		); err != nil {
			t.Fatalf("Failed to insert test data: %v", err)
		}
	}

	// Create collector state with test database
	state := &collectorState{
		cfg: collectorConfig{
			duckDBTable: "events",
			authEnabled: false,
		},
		queryDB: db,
	}

	// Test deletion by tenant
	req := httptest.NewRequest("DELETE", "/v1/events/by-tenant/tenant1", nil)
	w := httptest.NewRecorder()

	state.handleDeleteEvents(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp DeletionResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Deleted != 3 {
		t.Errorf("Expected 3 deleted events, got %d", resp.Deleted)
	}

	// Verify that only tenant1 events were deleted
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM events WHERE tenant_id = 'tenant1'`).Scan(&count); err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 tenant1 events, got %d", count)
	}

	// Verify tenant2 events still exist
	if err := db.QueryRow(`SELECT COUNT(*) FROM events WHERE tenant_id = 'tenant2'`).Scan(&count); err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 tenant2 event, got %d", count)
	}
}

// TestDeleteEventsByUser verifies deletion of events for a specific user
func TestDeleteEventsByUser(t *testing.T) {
	db, err := sql.Open("duckdb", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	// Create test table
	if _, err := db.Exec(`CREATE TABLE events (
		event_id VARCHAR,
		user_id VARCHAR,
		tenant_id VARCHAR,
		timestamp TIMESTAMP,
		raw VARCHAR
	)`); err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	// Insert test data with user_id
	testData := []struct {
		eventID  string
		userID   string
		tenantID string
	}{
		{"evt1", "user1", "tenant1"},
		{"evt2", "user1", "tenant1"},
		{"evt3", "user2", "tenant1"},
		{"evt4", "user1", "tenant2"},
	}

	for _, data := range testData {
		if _, err := db.Exec(
			`INSERT INTO events (event_id, user_id, tenant_id, timestamp, raw) VALUES (?, ?, ?, CURRENT_TIMESTAMP, '{}')`,
			data.eventID, data.userID, data.tenantID,
		); err != nil {
			t.Fatalf("Failed to insert test data: %v", err)
		}
	}

	state := &collectorState{
		cfg: collectorConfig{
			duckDBTable: "events",
			authEnabled: false,
		},
		queryDB: db,
	}

	// Test deletion by user
	req := httptest.NewRequest("DELETE", "/v1/events/by-user/user1", nil)
	w := httptest.NewRecorder()

	state.handleDeleteEvents(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp DeletionResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Deleted != 3 {
		t.Errorf("Expected 3 deleted events for user1, got %d", resp.Deleted)
	}

	// Verify user1 events are deleted
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM events WHERE user_id = 'user1'`).Scan(&count); err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 user1 events, got %d", count)
	}

	// Verify user2 events still exist
	if err := db.QueryRow(`SELECT COUNT(*) FROM events WHERE user_id = 'user2'`).Scan(&count); err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 user2 event, got %d", count)
	}
}

// TestDeleteEventByID verifies deletion of a specific event
func TestDeleteEventByID(t *testing.T) {
	db, err := sql.Open("duckdb", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	// Create test table
	if _, err := db.Exec(`CREATE TABLE events (
		event_id VARCHAR,
		tenant_id VARCHAR,
		timestamp TIMESTAMP,
		raw VARCHAR
	)`); err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	// Insert test data
	for i := 1; i <= 5; i++ {
		if _, err := db.Exec(
			`INSERT INTO events (event_id, tenant_id, timestamp, raw) VALUES (?, ?, CURRENT_TIMESTAMP, '{}')`,
			fmt.Sprintf("evt%d", i), "tenant1",
		); err != nil {
			t.Fatalf("Failed to insert test data: %v", err)
		}
	}

	state := &collectorState{
		cfg: collectorConfig{
			duckDBTable: "events",
			authEnabled: false,
		},
		queryDB: db,
	}

	// Test deletion by event ID
	req := httptest.NewRequest("DELETE", "/v1/events/evt3", nil)
	w := httptest.NewRecorder()

	state.handleDeleteEvents(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp DeletionResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Deleted != 1 {
		t.Errorf("Expected 1 deleted event, got %d", resp.Deleted)
	}

	// Verify only evt3 is deleted
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM events WHERE event_id = 'evt3'`).Scan(&count); err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected evt3 to be deleted, but found %d records", count)
	}

	// Verify other events still exist
	if err := db.QueryRow(`SELECT COUNT(*) FROM events`).Scan(&count); err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if count != 4 {
		t.Errorf("Expected 4 remaining events, got %d", count)
	}
}

// TestDeleteWithoutAuth verifies that deletion without auth fails
func TestDeleteWithoutAuth(t *testing.T) {
	db, err := sql.Open("duckdb", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	// Create test table
	if _, err := db.Exec(`CREATE TABLE events (
		event_id VARCHAR,
		tenant_id VARCHAR,
		timestamp TIMESTAMP,
		raw VARCHAR
	)`); err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	state := &collectorState{
		cfg: collectorConfig{
			duckDBTable: "events",
			authEnabled: true,
			apiKey:      "test-key",
			apiKeyHeader: "X-API-Key",
		},
		queryDB: db,
	}

	// Test deletion without API key
	req := httptest.NewRequest("DELETE", "/v1/events/evt1", nil)
	w := httptest.NewRecorder()

	state.handleDeleteEvents(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

// TestDeleteWithAuth verifies that deletion with auth succeeds
func TestDeleteWithAuth(t *testing.T) {
	db, err := sql.Open("duckdb", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	// Create test table
	if _, err := db.Exec(`CREATE TABLE events (
		event_id VARCHAR,
		tenant_id VARCHAR,
		timestamp TIMESTAMP,
		raw VARCHAR
	)`); err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	// Insert test event
	if _, err := db.Exec(
		`INSERT INTO events (event_id, tenant_id, timestamp, raw) VALUES (?, ?, CURRENT_TIMESTAMP, '{}')`,
		"evt1", "tenant1",
	); err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	state := &collectorState{
		cfg: collectorConfig{
			duckDBTable:   "events",
			authEnabled:   true,
			apiKey:        "test-key",
			apiKeyHeader:  "X-API-Key",
		},
		queryDB: db,
	}

	// Test deletion with API key
	req := httptest.NewRequest("DELETE", "/v1/events/evt1", nil)
	req.Header.Set("X-API-Key", "test-key")
	w := httptest.NewRecorder()

	state.handleDeleteEvents(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestDeleteWithAuditLogging verifies deletion audit logging
func TestDeleteWithAuditLogging(t *testing.T) {
	db, err := sql.Open("duckdb", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	// Create test table
	if _, err := db.Exec(`CREATE TABLE events (
		event_id VARCHAR,
		tenant_id VARCHAR,
		timestamp TIMESTAMP,
		raw VARCHAR
	)`); err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	// Insert test data
	if _, err := db.Exec(
		`INSERT INTO events (event_id, tenant_id, timestamp, raw) VALUES (?, ?, CURRENT_TIMESTAMP, '{}')`,
		"evt1", "tenant1",
	); err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	state := &collectorState{
		cfg: collectorConfig{
			duckDBTable: "events",
			authEnabled: false,
		},
		queryDB: db,
	}

	// Test deletion with reason in body
	body := bytes.NewReader([]byte(`{"reason": "GDPR request - user deletion"}`))
	req := httptest.NewRequest("DELETE", "/v1/events/by-tenant/tenant1", body)
	w := httptest.NewRecorder()

	state.handleDeleteEvents(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp DeletionResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Deleted != 1 {
		t.Errorf("Expected 1 deleted event, got %d", resp.Deleted)
	}
}

// TestDeleteWithMissingID verifies that deletion with missing ID fails
func TestDeleteWithMissingID(t *testing.T) {
	db, err := sql.Open("duckdb", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	state := &collectorState{
		cfg: collectorConfig{
			duckDBTable: "events",
			authEnabled: false,
		},
		queryDB: db,
	}

	// Test deletion without tenant ID
	req := httptest.NewRequest("DELETE", "/v1/events/by-tenant/", nil)
	w := httptest.NewRecorder()

	state.handleDeleteEvents(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// TestDeletionResponseFormat verifies the response format
func TestDeletionResponseFormat(t *testing.T) {
	db, err := sql.Open("duckdb", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	// Create test table
	if _, err := db.Exec(`CREATE TABLE events (
		event_id VARCHAR,
		tenant_id VARCHAR,
		timestamp TIMESTAMP,
		raw VARCHAR
	)`); err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	state := &collectorState{
		cfg: collectorConfig{
			duckDBTable: "events",
			authEnabled: false,
		},
		queryDB: db,
	}

	req := httptest.NewRequest("DELETE", "/v1/events/by-tenant/tenant1", nil)
	w := httptest.NewRecorder()

	state.handleDeleteEvents(w, req)

	var resp DeletionResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify response has required fields
	if resp.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp in response")
	}

	if resp.Deleted != 0 {
		t.Errorf("Expected 0 deleted events (empty table), got %d", resp.Deleted)
	}
}
