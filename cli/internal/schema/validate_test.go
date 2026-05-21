package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type conformanceManifest struct {
	Valid                  []string `json:"valid"`
	LooseOnlyValid         []string `json:"loose_only_valid"`
	Invalid                []string `json:"invalid"`
	StrictOnlyInvalid      []string `json:"strict_only_invalid"`
	InvalidIngest          []string `json:"invalid_ingest"`
	InvalidCollector       []string `json:"invalid_collector_response"`
	InvalidLimits          []string `json:"invalid_limits"`
	ManifestDirectory      string
}

func TestValidateSpecAssets(t *testing.T) {
	if err := ValidateSpecAssets(specRepoRoot(t)); err != nil {
		t.Fatalf("ValidateSpecAssets failed: %v", err)
	}
}

func TestValidateGoldenFixtures(t *testing.T) {
	root := specRepoRoot(t)
	manifest := loadConformanceManifest(t, root)

	validPaths := resolveFixturePaths(t, manifest.ManifestDirectory, manifest.Valid)
	if len(validPaths) == 0 {
		t.Fatal("expected valid fixtures")
	}
	for _, path := range validPaths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			if err := ValidateEventFile(path); err != nil {
				t.Fatalf("expected valid fixture to pass, got %v", err)
			}
		})
	}

	invalidPaths := resolveFixturePaths(t, manifest.ManifestDirectory,
		append(append(append(manifest.Invalid, manifest.StrictOnlyInvalid...), manifest.InvalidIngest...), manifest.InvalidCollector...),
	)
	if len(invalidPaths) == 0 {
		t.Fatal("expected invalid fixtures")
	}
	for _, path := range invalidPaths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			if err := ValidateEventFile(path); err == nil {
				t.Fatal("expected invalid fixture to fail")
			}
		})
	}

	for _, path := range resolveFixturePaths(t, manifest.ManifestDirectory, manifest.LooseOnlyValid) {
		path := path
		t.Run("strict_rejects_"+filepath.Base(path), func(t *testing.T) {
			if err := ValidateEventFile(path); err == nil {
				t.Fatal("expected loose-only valid fixture to fail strict validation")
			}
		})
	}

	if len(resolveFixturePaths(t, manifest.ManifestDirectory, manifest.InvalidLimits)) == 0 {
		t.Fatal("expected invalid_limits fixtures")
	}
}

func TestValidateWrappedAndNDJSON(t *testing.T) {
	wrapped := []byte(`{"events":[{"schema_version":"v1","event_version":"v1","timestamp":"2026-05-12T00:00:00Z","event_id":"evt_1","service":"checkout","event":"checkout.request","kind":"http"}]}`)
	if err := ValidateEventBytes(wrapped); err != nil {
		t.Fatalf("wrapped payload should pass: %v", err)
	}

	ndjson := []byte("{\"schema_version\":\"v1\",\"event_version\":\"v1\",\"timestamp\":\"2026-05-12T00:00:00Z\",\"event_id\":\"evt_1\",\"service\":\"checkout\",\"event\":\"checkout.request\",\"kind\":\"http\"}\n{\"schema_version\":\"v1\",\"event_version\":\"v1\",\"timestamp\":\"2026-05-12T00:01:00Z\",\"event_id\":\"evt_2\",\"service\":\"checkout\",\"event\":\"checkout.request\",\"kind\":\"http\"}\n")
	if err := ValidateEventBytes(ndjson); err != nil {
		t.Fatalf("ndjson payload should pass: %v", err)
	}
}

func specRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "spec"))
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("spec repo path not found: %v", err)
	}
	return root
}

func loadConformanceManifest(t *testing.T, root string) conformanceManifest {
	t.Helper()
	manifestPath := filepath.Join(root, "conformance", "manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		manifestPath = filepath.Join(root, "examples", "golden", "manifest.json")
	}
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read conformance manifest: %v", err)
	}
	var manifest conformanceManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("decode conformance manifest: %v", err)
	}
	manifest.ManifestDirectory = filepath.Dir(manifestPath)
	return manifest
}

func resolveFixturePaths(t *testing.T, manifestDir string, rels []string) []string {
	t.Helper()
	paths := make([]string, 0, len(rels))
	for _, rel := range rels {
		path := filepath.Clean(filepath.Join(manifestDir, rel))
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("fixture path not found: %s", path)
		}
		paths = append(paths, path)
	}
	return paths
}
