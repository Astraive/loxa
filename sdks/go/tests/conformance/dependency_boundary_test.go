package conformance

import (
	"os/exec"
	"strings"
	"testing"
)

func TestRootModuleDependencyBoundary(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "github.com/astraive/loxa/sdks/go")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list deps failed: %v\n%s", err, string(out))
	}

	disallowed := []string{
		"github.com/gin-gonic/gin",
		"github.com/go-chi/chi",
		"github.com/gofiber/fiber",
		"github.com/labstack/echo",
		"github.com/ClickHouse/clickhouse-go",
		"github.com/aws/aws-sdk-go-v2/service/s3",
		"cloud.google.com/go/storage",
		"github.com/jackc/pgx",
		"github.com/twmb/franz-go",
		"github.com/marcboeker/go-duckdb",
	}

	deps := string(out)
	for _, bad := range disallowed {
		if strings.Contains(deps, bad) {
			t.Fatalf("root dependency boundary violated: found disallowed dependency %q", bad)
		}
	}
}
