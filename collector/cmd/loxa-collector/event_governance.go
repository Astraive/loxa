package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	speccontract "github.com/astraive/loxa/spec/generated/go/contract"
)

var secretValuePattern = regexp.MustCompile(`(?i)(bearer\s+[a-z0-9._~+/=-]{12,}|sk-[a-z0-9]{16,}|-----BEGIN [A-Z ]*PRIVATE KEY-----)`)

type governanceError struct {
	Code      string
	Message   string
	Retryable bool
}

func (e governanceError) Error() string { return e.Message }

func (s *collectorState) prepareEvent(raw []byte) ([]byte, *governanceError) {
	if s.limitsEnabled() && s.cfg.maxEventBytes > 0 && int64(len(raw)) > s.cfg.maxEventBytes {
		return raw, &governanceError{
			Code:    "event_too_large",
			Message: fmt.Sprintf("event exceeds max_event_bytes (%d > %d)", len(raw), s.cfg.maxEventBytes),
		}
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return raw, &governanceError{Code: "invalid_json", Message: err.Error()}
	}

	if err := s.enforcePayloadLimits(payload); err != nil {
		return raw, err
	}

	modified := false
	if normalizeCanonicalAliases(payload) {
		modified = true
	}
	if s.applyIdentity(payload) {
		modified = true
	}
	if s.applyCollectorRedaction(payload) {
		modified = true
	}
	if !modified {
		return raw, nil
	}
	next, err := json.Marshal(payload)
	if err != nil {
		return raw, &governanceError{Code: "event_rewrite_failed", Message: err.Error(), Retryable: true}
	}
	if s.limitsEnabled() && s.cfg.maxEventBytes > 0 && int64(len(next)) > s.cfg.maxEventBytes {
		return raw, &governanceError{
			Code:    "event_too_large_after_governance",
			Message: fmt.Sprintf("event exceeds max_event_bytes after collector governance (%d > %d)", len(next), s.cfg.maxEventBytes),
		}
	}
	return next, nil
}

func normalizeCanonicalAliases(payload map[string]any) bool {
	return speccontract.NormalizeEventAliasesMap(payload)
}

func (s *collectorState) enforcePayloadLimits(payload map[string]any) *governanceError {
	if !s.limitsEnabled() {
		return nil
	}
	if s.cfg.maxAttrCount > 0 {
		if count := countFields(payload); count > s.cfg.maxAttrCount {
			return &governanceError{
				Code:    "attr_count_exceeded",
				Message: fmt.Sprintf("event field count exceeds max_attr_count (%d > %d)", count, s.cfg.maxAttrCount),
			}
		}
	}
	if s.cfg.maxAttrDepth > 0 {
		if depth := maxDepth(payload); depth > s.cfg.maxAttrDepth {
			return &governanceError{
				Code:    "attr_depth_exceeded",
				Message: fmt.Sprintf("event depth exceeds max_attr_depth (%d > %d)", depth, s.cfg.maxAttrDepth),
			}
		}
	}
	if s.cfg.maxStringLength > 0 {
		if path, length, ok := firstStringLongerThan(payload, s.cfg.maxStringLength, ""); ok {
			return &governanceError{
				Code:    "string_too_long",
				Message: fmt.Sprintf("field %s exceeds max_string_length (%d > %d)", path, length, s.cfg.maxStringLength),
			}
		}
	}
	return nil
}

func (s *collectorState) applyIdentity(payload map[string]any) bool {
	if !s.identityEnabled() {
		return false
	}
	if !s.cfg.authIdentityWins || s.cfg.allowPayloadIdentity {
		return false
	}

	modified := false
	if s.cfg.boundServiceName != "" {
		payload["service"] = s.cfg.boundServiceName
		serviceObj := objectAt(payload, "service_identity")
		serviceObj["name"] = s.cfg.boundServiceName
		modified = true
		if s.cfg.boundServiceVersion != "" {
			serviceObj["version"] = s.cfg.boundServiceVersion
		}
	}
	if s.cfg.boundEnvironment != "" || s.cfg.boundRegion != "" {
		deployment := objectAt(payload, "deployment")
		if s.cfg.boundEnvironment != "" {
			deployment["environment"] = s.cfg.boundEnvironment
		}
		if s.cfg.boundRegion != "" {
			deployment["region"] = s.cfg.boundRegion
		}
		modified = true
	}
	if s.cfg.boundTenantID != "" {
		objectAt(payload, "tenant")["id"] = s.cfg.boundTenantID
		modified = true
	}
	if s.cfg.boundWorkspaceID != "" {
		payload["workspace"] = map[string]any{"id": s.cfg.boundWorkspaceID}
		modified = true
	}
	if s.cfg.boundOrganizationID != "" {
		payload["organization"] = map[string]any{"id": s.cfg.boundOrganizationID}
		modified = true
	}
	return modified
}

func (s *collectorState) applyCollectorRedaction(payload map[string]any) bool {
	if !s.redactionEnabled() {
		return false
	}
	mode := strings.ToLower(strings.TrimSpace(s.cfg.privacyMode))
	if mode == "off" || (!s.cfg.collectorRedaction && !s.cfg.emergencyRedaction) {
		return false
	}
	blocked := make(map[string]struct{}, len(s.cfg.privacyBlocklist))
	for _, key := range s.cfg.privacyBlocklist {
		blocked[normalizeSensitiveKey(key)] = struct{}{}
	}
	if len(blocked) == 0 && !s.cfg.secretScan {
		return false
	}
	return redactObject(payload, blocked, s.cfg.secretScan)
}

func redactObject(value any, blocked map[string]struct{}, scanSecrets bool) bool {
	modified := false
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if _, ok := blocked[normalizeSensitiveKey(key)]; ok {
				typed[key] = "[REDACTED]"
				modified = true
				continue
			}
			if scanSecrets {
				if text, ok := child.(string); ok && secretValuePattern.MatchString(text) {
					typed[key] = "[REDACTED]"
					modified = true
					continue
				}
			}
			if redactObject(child, blocked, scanSecrets) {
				modified = true
			}
		}
	case []any:
		for _, child := range typed {
			if redactObject(child, blocked, scanSecrets) {
				modified = true
			}
		}
	}
	return modified
}

func normalizeSensitiveKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.ReplaceAll(key, "-", "_")
	key = strings.ReplaceAll(key, ".", "_")
	return key
}

func objectAt(payload map[string]any, key string) map[string]any {
	if existing, ok := payload[key].(map[string]any); ok {
		return existing
	}
	next := map[string]any{}
	payload[key] = next
	return next
}

func countFields(value any) int {
	switch typed := value.(type) {
	case map[string]any:
		total := len(typed)
		for _, child := range typed {
			total += countFields(child)
		}
		return total
	case []any:
		total := 0
		for _, child := range typed {
			total += countFields(child)
		}
		return total
	default:
		return 0
	}
}

func maxDepth(value any) int {
	switch typed := value.(type) {
	case map[string]any:
		maxChild := 0
		for _, child := range typed {
			if depth := maxDepth(child); depth > maxChild {
				maxChild = depth
			}
		}
		return 1 + maxChild
	case []any:
		maxChild := 0
		for _, child := range typed {
			if depth := maxDepth(child); depth > maxChild {
				maxChild = depth
			}
		}
		return 1 + maxChild
	default:
		return 1
	}
}

func firstStringLongerThan(value any, limit int, path string) (string, int, bool) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}
			if foundPath, length, ok := firstStringLongerThan(child, limit, childPath); ok {
				return foundPath, length, true
			}
		}
	case []any:
		for i, child := range typed {
			childPath := fmt.Sprintf("%s[%d]", path, i)
			if foundPath, length, ok := firstStringLongerThan(child, limit, childPath); ok {
				return foundPath, length, true
			}
		}
	case string:
		if len(typed) > limit {
			if path == "" {
				path = "<root>"
			}
			return path, len(typed), true
		}
	}
	return "", 0, false
}
