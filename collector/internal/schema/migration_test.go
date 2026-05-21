package schema_test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEventVersionMigrationSkippedIfNoFixtures(t *testing.T) {
	fixturesDir := filepath.Join("..", "..", "..", "loxa-spec", "examples", "golden", "migrations")
	if _, err := os.Stat(fixturesDir); os.IsNotExist(err) {
		t.Skip("no migration fixtures present; skipping migration tests")
	}
}
