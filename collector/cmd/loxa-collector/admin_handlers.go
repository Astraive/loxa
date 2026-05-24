package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"

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
	env := strings.TrimSpace(asString(req["env"]))
	if env == "" {
		env = "live"
	}
	kind := strings.TrimSpace(asString(req["kind"]))
	if kind == "" {
		kind = "sec"
	}
	keyID := "k_" + randomToken(8)
	secret := randomToken(24)
	key := "lx_" + kind + "_" + env + "_" + keyID + "_" + secret
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
	writeJSON(w, http.StatusOK, map[string]any{"id": r.PathValue("id"), "revoked": true})
}

func (s *collectorState) handleKeyRotate(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "auth_failed"})
		return
	}
	keyID := r.PathValue("id")
	if strings.TrimSpace(keyID) == "" {
		keyID = "k_" + randomToken(8)
	}
	secret := randomToken(24)
	writeJSON(w, http.StatusOK, map[string]any{
		"id":      keyID,
		"key_id":  keyID,
		"key":     "lx_sec_live_" + keyID + "_" + secret,
		"rotated": true,
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

func randomToken(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "local"
	}
	return strings.TrimRight(base64.RawURLEncoding.EncodeToString(buf), "=")
}
