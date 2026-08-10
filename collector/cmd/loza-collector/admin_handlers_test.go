package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/astraive/loza/collector/internal/auth"
)

func newTestCollectorState() *collectorState {
	serverSecret := []byte("test-server-secret-32bytes!!!!!")
	store := &memoryKeyStore{keys: make(map[string]*auth.KeyRecord)}
	return &collectorState{
		cfg: collectorConfig{
			authEnabled: false, // bypass auth for handler tests
		},
		keyStore:     store,
		serverSecret: serverSecret,
	}
}

func TestHandleKeyCreate_StoresKeyWithCorrectHash(t *testing.T) {
	state := newTestCollectorState()

	body := map[string]any{
		"kind": "sec",
		"env":  "live",
		"name": "test-key",
	}
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/admin/keys", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	state.handleKeyCreate(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	keyID, _ := resp["key_id"].(string)
	if keyID == "" {
		t.Fatal("expected key_id in response")
	}

	// Verify the key was stored with a valid SecretHash
	stored, err := state.keyStore.FindByKeyID(context.Background(), keyID)
	if err != nil {
		t.Fatalf("find key: %v", err)
	}
	if stored == nil {
		t.Fatal("key not found in store")
	}
	if stored.SecretHash == nil {
		t.Fatal("SecretHash is nil")
	}
	if stored.Kind != auth.KeyKindSecret {
		t.Errorf("kind = %q, want %q", stored.Kind, auth.KeyKindSecret)
	}
}

func TestHandleKeyCreate_CreatedKeyCanAuthenticate(t *testing.T) {
	state := newTestCollectorState()

	// Create a key
	body := map[string]any{"kind": "sec", "env": "live"}
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/admin/keys", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	state.handleKeyCreate(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create key: expected 201, got %d", rec.Code)
	}

	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	fullKey, _ := resp["key"].(string)
	keyID, _ := resp["key_id"].(string)

	if fullKey == "" || keyID == "" {
		t.Fatal("expected key and key_id in response")
	}

	// Extract the secret from the key: format is lx_{kind}_{env}_{keyID}_{secret}
	// The secret is the last underscore-separated segment
	keyParts := strings.Split(fullKey, "_")
	if len(keyParts) < 5 {
		t.Fatalf("unexpected key format: %s (parts: %d)", fullKey, len(keyParts))
	}
	rawSecret := keyParts[len(keyParts)-1] // the actual secret token

	stored, _ := state.keyStore.FindByKeyID(context.Background(), keyID)
	if stored == nil {
		t.Fatal("key not in store")
	}

	// Verify the stored hash was created from the raw secret
	computedHash := auth.HashSecret(rawSecret, state.serverSecret)
	if !auth.CompareSecret(computedHash, stored.SecretHash) {
		t.Fatal("stored hash does not match computed hash from key secret")
	}
}

func TestHandleKeyRevoke_InvalidatesKey(t *testing.T) {
	state := newTestCollectorState()

	// Create a key first
	body := map[string]any{"kind": "sec", "env": "live"}
	payload, _ := json.Marshal(body)
	createReq := httptest.NewRequest(http.MethodPost, "/admin/keys", bytes.NewReader(payload))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	state.handleKeyCreate(createRec, createReq)

	var createResp map[string]any
	_ = json.Unmarshal(createRec.Body.Bytes(), &createResp)
	keyID, _ := createResp["key_id"].(string)

	// Revoke it
	revokeReq := httptest.NewRequest(http.MethodDelete, "/admin/keys/"+keyID, nil)
	revokeReq.SetPathValue("id", keyID)
	revokeRec := httptest.NewRecorder()
	state.handleKeyRevoke(revokeRec, revokeReq)

	if revokeRec.Code != http.StatusOK {
		t.Fatalf("revoke: expected 200, got %d", revokeRec.Code)
	}

	// Verify key is revoked
	stored, _ := state.keyStore.FindByKeyID(context.Background(), keyID)
	if stored == nil {
		t.Fatal("key should still exist in store after revoke")
	}
	if stored.RevokedAt == nil {
		t.Fatal("expected RevokedAt to be set")
	}
}

func TestHandleKeyRotate_CreatesNewAndRevokesOld(t *testing.T) {
	state := newTestCollectorState()

	// Create initial key
	body := map[string]any{"kind": "sec", "env": "live"}
	payload, _ := json.Marshal(body)
	createReq := httptest.NewRequest(http.MethodPost, "/admin/keys", bytes.NewReader(payload))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	state.handleKeyCreate(createRec, createReq)

	var createResp map[string]any
	_ = json.Unmarshal(createRec.Body.Bytes(), &createResp)
	oldKeyID, _ := createResp["key_id"].(string)

	// Rotate it
	rotateReq := httptest.NewRequest(http.MethodPost, "/admin/keys/"+oldKeyID+"/rotate", nil)
	rotateReq.SetPathValue("id", oldKeyID)
	rotateRec := httptest.NewRecorder()
	state.handleKeyRotate(rotateRec, rotateReq)

	if rotateRec.Code != http.StatusOK {
		t.Fatalf("rotate: expected 200, got %d: %s", rotateRec.Code, rotateRec.Body.String())
	}

	var rotateResp map[string]any
	_ = json.Unmarshal(rotateRec.Body.Bytes(), &rotateResp)

	newKeyID, _ := rotateResp["key_id"].(string)
	if newKeyID == "" {
		t.Fatal("expected new key_id in rotate response")
	}
	if newKeyID == oldKeyID {
		t.Fatal("rotated key_id should differ from original")
	}

	// Old key should be revoked
	oldRec, _ := state.keyStore.FindByKeyID(context.Background(), oldKeyID)
	if oldRec == nil {
		t.Fatal("old key should still exist")
	}
	if oldRec.RevokedAt == nil {
		t.Fatal("old key should be revoked after rotation")
	}

	// New key should exist and not be revoked
	newRec, _ := state.keyStore.FindByKeyID(context.Background(), newKeyID)
	if newRec == nil {
		t.Fatal("new key should exist in store")
	}
	if newRec.RevokedAt != nil {
		t.Fatal("new key should not be revoked")
	}
}

func TestHandleKeyCreate_InvalidKindDefaultsToSec(t *testing.T) {
	state := newTestCollectorState()

	body := map[string]any{"kind": "", "env": "live"}
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/admin/keys", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	state.handleKeyCreate(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["kind"] != "sec" {
		t.Errorf("default kind = %q, want %q", resp["kind"], "sec")
	}
}

func TestHandleKeyCreate_InvalidEnvDefaultsToLive(t *testing.T) {
	state := newTestCollectorState()

	body := map[string]any{"kind": "sec", "env": ""}
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/admin/keys", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	state.handleKeyCreate(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["env"] != "live" {
		t.Errorf("default env = %q, want %q", resp["env"], "live")
	}
}

func TestHandleKeyRevoke_NotFound(t *testing.T) {
	state := newTestCollectorState()

	req := httptest.NewRequest(http.MethodDelete, "/admin/keys/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	rec := httptest.NewRecorder()
	state.handleKeyRevoke(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}
