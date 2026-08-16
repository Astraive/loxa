package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/marcboeker/go-duckdb"
)

func TestLQLStdioCompilerHandshakeAndScope(t *testing.T) {
	binary := os.Getenv("LOZA_LQL_BINARY")
	if binary == "" {
		candidates := []string{
			filepath.Join("..", "..", "..", "..", "lql", "target", "debug", "lql.exe"),
			filepath.Join("..", "..", "..", "lql", "target", "debug", "lql.exe"),
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				binary = candidate
				break
			}
		}
	}
	if binary == "" {
		t.Skip("LQL binary unavailable")
	}
	cfg := collectorConfig{
		lqlBinary: binary, lqlExpectedProtocol: 1, lqlExpectedCompiler: "0.4.0",
		lqlExpectedLanguage: "0.1", lqlStartupTimeout: 5e9, lqlCompileTimeout: 5e9,
		lqlMaxConcurrent: 2,
	}
	compiler, err := newLQLStdioCompiler(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer compiler.Close(context.Background())
	plan, err := compiler.Compile(context.Background(), LQLCompileRequest{
		Source: "from events | where level = \"error\"", Target: "duckdb", Limit: 10,
		Scope: LQLScope{Collector: "acme", Environment: "prod"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.SQL == "" || len(plan.Args) == 0 {
		t.Fatalf("expected parameterized plan, got %#v", plan)
	}
	if len(plan.Args) < 3 {
		t.Fatalf("expected query, scope, and limit bindings, got %#v", plan.Args)
	}
	db, err := sql.Open("duckdb", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE events (level VARCHAR, collector VARCHAR, environment VARCHAR);
		INSERT INTO events VALUES ('error', 'acme', 'prod'), ('info', 'acme', 'prod'), ('error', 'other', 'prod')`); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query(plan.SQL, plan.Args...)
	if err != nil {
		t.Fatalf("execute compiled plan: %v\nSQL: %s\nargs: %#v", err, plan.SQL, plan.Args)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		count++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected one scoped error event, got %d", count)
	}
}
