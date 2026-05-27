package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/astraive/loxa-collector/internal/auth"
	speccontract "github.com/astraive/loxa/spec/generated/go/contract"
)

func (s *collectorState) handleValidate(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "auth_failed"})
		return
	}
	payload, ok := decodeJSONMap(w, r, "invalid_validate_request")
	if !ok {
		return
	}
	if err := validateCollectorPayload(payload, false); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"valid": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"valid": true})
}

func (s *collectorState) handleSchemaCheck(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "auth_failed"})
		return
	}
	payload, ok := decodeJSONMap(w, r, "invalid_schema_check_request")
	if !ok {
		return
	}
	if err := validateCollectorPayload(payload, false); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"valid": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"valid":          true,
		"schema_version": s.cfg.schemaSchemaVersion,
		"event_version":  s.cfg.schemaEventVersion,
	})
}

func (s *collectorState) handlePolicyValidate(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "auth_failed"})
		return
	}
	policy, ok := decodeJSONMap(w, r, "invalid_policy_request")
	if !ok {
		return
	}
	errors := validatePolicyMap(policy)
	writeJSON(w, statusForValidation(errors), map[string]any{
		"valid":  len(errors) == 0,
		"errors": errors,
	})
}

func (s *collectorState) handleRetentionApply(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "auth_failed"})
		return
	}
	if err := s.executeRetention(); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"applied": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"applied": true})
}

func (s *collectorState) handleKeyCreate(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "auth_failed"})
		return
	}
	req, ok := decodeJSONMap(w, r, "invalid_key_create_request")
	if !ok {
		return
	}
	kind := strings.TrimSpace(asString(req["kind"]))
	if kind == "" {
		kind = "sec"
	}
	if kind != "sec" && kind != "pub" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_kind", "message": "kind must be 'sec' or 'pub'"})
		return
	}
	env := strings.TrimSpace(asString(req["env"]))
	if env == "" {
		env = "live"
	}
	if env != "live" && env != "test" && env != "dev" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_env", "message": "env must be 'live', 'test', or 'dev'"})
		return
	}
	token8, err := randomToken(8)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "token_generation_failed"})
		return
	}
	token24, err := randomToken(24)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "token_generation_failed"})
		return
	}
	keyID := "k" + token8
	secret := token24
	key := "lx_" + kind + "_" + env + "_" + keyID + "_" + secret

	// Store the new key in the key store so it can authenticate requests
	if s.keyStore != nil {
		secretHash := auth.HashSecret(secret, s.serverSecret)
		keyKind := auth.KeyKindSecret
		if kind == "pub" {
			keyKind = auth.KeyKindPublic
		}
		newKey := &auth.KeyRecord{
			ID:           keyID,
			KeyID:        keyID,
			SecretHash:   secretHash,
			Kind:         keyKind,
			Roles:        []auth.Role{auth.RoleIngestServer},
			AllowedEnvs:  []string{env},
		}
		if err := s.keyStore.CreateKey(newKey); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "key_store_error"})
			return
		}
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":     keyID,
		"key_id": keyID,
		"key":    key,
		"kind":   kind,
		"env":    env,
		"name":   asString(req["name"]),
	})
}

func (s *collectorState) handleKeyRevoke(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "auth_failed"})
		return
	}
	keyID := r.PathValue("id")
	if s.keyStore == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "key_store_not_configured"})
		return
	}
	if err := s.keyStore.RevokeKey(keyID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "key_not_found", "id": keyID})
		return
	}
	if s.keyCache != nil {
		s.keyCache.Invalidate(keyID)
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": keyID, "revoked": true})
}

func (s *collectorState) handleKeyRotate(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "auth_failed"})
		return
	}
	oldKeyID := r.PathValue("id")
	if strings.TrimSpace(oldKeyID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing_key_id"})
		return
	}
	if s.keyStore == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "key_store_not_configured"})
		return
	}

	// Look up old key to preserve kind/env
	oldRecord, err := s.keyStore.FindByKeyID(r.Context(), oldKeyID)
	if err != nil || oldRecord == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "key_not_found", "id": oldKeyID})
		return
	}
	kind := string(oldRecord.Kind)
	env := "live"
	if len(oldRecord.AllowedEnvs) > 0 {
		env = oldRecord.AllowedEnvs[0]
	}

	// Generate a NEW keyID for the rotated key
	token8, err := randomToken(8)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "token_generation_failed"})
		return
	}
	newKeyID := "k" + token8
	newSecret, err := randomToken(24)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "token_generation_failed"})
		return
	}
	// Store the NEW key FIRST, then revoke old (order matters: never leave zero keys)
	secretHash := auth.HashSecret(newSecret, s.serverSecret)
	newKey := &auth.KeyRecord{
		ID:           newKeyID,
		KeyID:        newKeyID,
		SecretHash:   secretHash,
		Kind:         oldRecord.Kind,
		Roles:        []auth.Role{auth.RoleIngestServer},
		AllowedEnvs:  []string{env},
	}
	if err := s.keyStore.CreateKey(newKey); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "key_store_error"})
		return
	}
	// Revoke old key AFTER new key is stored
	_ = s.keyStore.RevokeKey(oldKeyID)

	// Invalidate cache for old key
	if s.keyCache != nil {
		s.keyCache.Invalidate(oldKeyID)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":         newKeyID,
		"key_id":     newKeyID,
		"key":        "lx_" + kind + "_" + env + "_" + newKeyID + "_" + newSecret,
		"rotated":    true,
		"old_key_id": oldKeyID,
	})
}

func decodeJSONMap(w http.ResponseWriter, r *http.Request, code string) (map[string]any, bool) {
	var payload map[string]any
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": code, "message": err.Error()})
		return nil, false
	}
	return payload, true
}

func validateCollectorPayload(payload map[string]any, strict bool) error {
	if _, ok := payload["events"]; ok {
		return speccontract.ValidateIngestEnvelopeMap(payload, strict)
	}
	return speccontract.ValidateEventMap(payload, strict)
}

func validatePolicyMap(policy map[string]any) []string {
	var errors []string
	for _, key := range []string{"max_attr_length", "max_event_bytes", "max_attrs"} {
		if value, ok := policy[key]; ok && numeric(value) < 0 {
			errors = append(errors, key+" must be non-negative")
		}
	}
	if rate, ok := policy["sample_rate"]; ok {
		value := numeric(rate)
		if value < 0 || value > 1 {
			errors = append(errors, "sample_rate must be between 0 and 1")
		}
	}
	return errors
}

func statusForValidation(errors []string) int {
	if len(errors) == 0 {
		return http.StatusOK
	}
	return http.StatusBadRequest
}

func numeric(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	default:
		return 0
	}
}

func asString(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("crypto/rand.Read failed: %w", err)
	}
	// Use base64 RawStdEncoding (uses + and /, not _) to avoid
	// conflicts with the _ separator in the key format lx_{kind}_{env}_{keyID}_{secret}.
	return strings.TrimRight(base64.RawStdEncoding.EncodeToString(buf), "="), nil
}
