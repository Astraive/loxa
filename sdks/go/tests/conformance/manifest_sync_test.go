package conformance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type parityManifest struct {
	Version         string   `json:"version"`
	Stability       string   `json:"stability"`
	Lifecycle       []string `json:"lifecycle"`
	ExcludedFromSDK []string `json:"excluded_from_sdk"`
}

func TestParityManifestMatchesStableLifecycleDoc(t *testing.T) {
	manifestPath := filepath.Join("..", "..", "..", "..", "spec", "docs", "sdk-parity-manifest.json")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read parity manifest: %v", err)
	}
	var manifest parityManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("unmarshal parity manifest: %v", err)
	}
	if manifest.Version != "1.0.0" {
		t.Fatalf("expected stable manifest version 1.0.0, got %q", manifest.Version)
	}
	if manifest.Stability != "stable-v1" {
		t.Fatalf("expected stability stable-v1, got %q", manifest.Stability)
	}

	docPath := filepath.Join("..", "..", "docs", "public-api.md")
	docRaw, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read public API doc: %v", err)
	}
	doc := string(docRaw)

	for _, name := range manifest.Lifecycle {
		if !strings.Contains(doc, "`"+name+"`") {
			t.Fatalf("stable lifecycle API %q missing from docs/public-api.md", name)
		}
	}
	for _, excluded := range []string{"Kafka", "DuckDB", "ClickHouse", "Postgres", "Loki", "OTLP", "S3", "GCS"} {
		found := false
		for _, got := range manifest.ExcludedFromSDK {
			if got == excluded {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected %q in excluded_from_sdk", excluded)
		}
	}
}
