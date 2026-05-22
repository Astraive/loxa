package packages_test

import (
	"testing"

	"github.com/astraive/loxa/sdks/go/src/packages"
)

func TestCatalogsNonEmpty(t *testing.T) {
	if len(packages.Middleware()) == 0 {
		t.Fatalf("expected middleware catalog entries")
	}
	if len(packages.Integrations()) == 0 {
		t.Fatalf("expected integration catalog entries")
	}
	if len(packages.Sinks()) == 0 {
		t.Fatalf("expected sink catalog entries")
	}
}
