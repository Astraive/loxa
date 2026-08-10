package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	speccontract "github.com/astraive/loza/spec/generated/go/contract"
)

func firstExisting(paths ...string) string {
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return paths[0]
}

func ValidateSpecAssets(specRepoPath string) error {
	requiredPaths := []string{
		firstExisting(
			filepath.Join(specRepoPath, "spec", "schemas", "json", "event.schema.json"),
			filepath.Join(specRepoPath, "schema", "event.schema.json"),
		),
		firstExisting(
			filepath.Join(specRepoPath, "spec", "schemas", "json", "event.strict.schema.json"),
			filepath.Join(specRepoPath, "schema", "event.strict.schema.json"),
		),
		firstExisting(
			filepath.Join(specRepoPath, "spec", "schemas", "json", "ingest-envelope.schema.json"),
			filepath.Join(specRepoPath, "schema", "ingest.schema.json"),
		),
		firstExisting(
			filepath.Join(specRepoPath, "spec", "schemas", "json", "collector-response.schema.json"),
			filepath.Join(specRepoPath, "schema", "collector-response.schema.json"),
		),
		firstExisting(
			filepath.Join(specRepoPath, "spec", "openapi", "collector.openapi.yaml"),
			filepath.Join(specRepoPath, "openapi", "collector.openapi.yaml"),
		),
		firstExisting(
			filepath.Join(specRepoPath, "spec", "proto", "loza", "core", "event.proto"),
			filepath.Join(specRepoPath, "proto", "loza", "core", "event.proto"),
			filepath.Join(specRepoPath, "releases", "v1", "proto", "loza", "core", "event.proto"),
		),
		firstExisting(
			filepath.Join(specRepoPath, "spec", "proto", "loza", "core", "ingest.proto"),
			filepath.Join(specRepoPath, "proto", "loza", "core", "ingest.proto"),
			filepath.Join(specRepoPath, "releases", "v1", "proto", "loza", "core", "ingest.proto"),
		),
		firstExisting(
			filepath.Join(specRepoPath, "spec", "proto", "loza", "core", "collector.proto"),
			filepath.Join(specRepoPath, "proto", "loza", "core", "collector.proto"),
			filepath.Join(specRepoPath, "releases", "v1", "proto", "loza", "core", "collector.proto"),
		),
		filepath.Join(specRepoPath, "generated", "go", "contract", "contract.go"),
		firstExisting(
			filepath.Join(specRepoPath, "generated", "contract", "conformance_manifest.json"),
			filepath.Join(specRepoPath, "generated", "conformance_manifest.json"),
		),
		firstExisting(
			filepath.Join(specRepoPath, "fixtures", "valid"),
			filepath.Join(specRepoPath, "examples", "golden", "valid"),
		),
		firstExisting(
			filepath.Join(specRepoPath, "fixtures", "invalid"),
			filepath.Join(specRepoPath, "examples", "golden", "invalid"),
		),
	}

	for _, requiredPath := range requiredPaths {
		if _, err := os.Stat(requiredPath); err != nil {
			return fmt.Errorf("missing spec asset %s: %w", requiredPath, err)
		}
	}
	return nil
}

func ValidateEventFile(path string) error {
	return ValidateEventFileStrict(path, true)
}

func ValidateEventFileStrict(path string, strict bool) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return ValidateEventBytesStrict(raw, strict)
}

func ValidateEventBytes(raw []byte) error {
	return ValidateEventBytesStrict(raw, true)
}

func ValidateEventBytesStrict(raw []byte, strict bool) error {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err == nil {
		if _, ok := payload["request_id"]; ok {
			if _, hasStatus := payload["status"]; hasStatus {
				return speccontract.ValidateCollectorResponseMap(payload)
			}
		}
		if _, hasStatus := payload["status"]; hasStatus {
			if _, hasAcks := payload["acks"]; hasAcks {
				return speccontract.ValidateCollectorResponseMap(payload)
			}
			if _, hasAccepted := payload["accepted"]; hasAccepted {
				return speccontract.ValidateCollectorResponseMap(payload)
			}
		}
		if _, ok := payload["request_id"]; ok {
			return speccontract.ValidateFlexibleJSONBytes(raw, strict)
		}
		if len(raw) > speccontract.MaxEventBytes && payload["events"] == nil {
			return fmt.Errorf("payload exceeds max_event_size_bytes (%d > %d)", len(raw), speccontract.MaxEventBytes)
		}
		if eventsRaw, ok := payload["events"].([]any); ok && payload["api_version"] == nil {
			for _, item := range eventsRaw {
				event, ok := item.(map[string]any)
				if !ok {
					return fmt.Errorf("wrapped events must be JSON objects")
				}
				if err := speccontract.ValidateEventMap(event, strict); err != nil {
					return err
				}
			}
			return nil
		}
	}
	return speccontract.ValidateFlexibleJSONBytes(raw, strict)
}
