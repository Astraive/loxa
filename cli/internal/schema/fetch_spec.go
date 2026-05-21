package schema

import (
	"os"
	"path/filepath"
)

func firstExistingPath(paths ...string) string {
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return paths[0]
}

func EventSchemaPath(specRepoPath string) string {
	return firstExistingPath(
		filepath.Join(specRepoPath, "spec", "schemas", "json", "event.schema.json"),
		filepath.Join(specRepoPath, "schema", "event.schema.json"),
	)
}

func FetchSpec(specRepoPath string) error {
	return ValidateSpecAssets(specRepoPath)
}
