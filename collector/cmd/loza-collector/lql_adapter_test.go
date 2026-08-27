package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/marcboeker/go-duckdb"
)

func testLQLBinary(t *testing.T) string {
	t.Helper()
	if binary := os.Getenv("LOZA_LQL_BINARY"); binary != "" {
		return binary
	}
	candidates := []string{
		filepath.Join("..", "..", "..", "..", "lql", "target", "debug", "lql.exe"),
		filepath.Join("..", "..", "..", "lql", "target", "debug", "lql.exe"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	t.Skip("LQL binary unavailable")
	return ""
}

func testLQLConfig(binary string) collectorConfig {
	return collectorConfig{
		lqlBinary: binary, lqlExpectedProtocol: 1, lqlExpectedCompiler: "0.5.0",
		lqlExpectedLanguage: "0.1", lqlStartupTimeout: 5e9, lqlCompileTimeout: 5e9,
		lqlMaxConcurrent: 1,
	}
}

func TestLQLStdioCompilerRejectsConcurrentRequests(t *testing.T) {
	_, err := newLQLStdioCompiler(context.Background(), collectorConfig{
		lqlBinary:        "lql",
		lqlMaxConcurrent: 2,
	})
	if err == nil || !strings.Contains(err.Error(), "exactly one concurrent request") {
		t.Fatalf("expected sequential stdio validation error, got %v", err)
	}
}

func TestLQLStdioCompilerHandshakeAndScope(t *testing.T) {
	cfg := testLQLConfig(testLQLBinary(t))
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

func TestLQLStdioCompilerRestartsAfterProcessLoss(t *testing.T) {
	compiler, err := newLQLStdioCompiler(context.Background(), testLQLConfig(testLQLBinary(t)))
	if err != nil {
		t.Fatal(err)
	}
	defer compiler.Close(context.Background())

	compiler.mu.Lock()
	previousPID := compiler.cmd.Process.Pid
	_ = compiler.killLocked()
	compiler.mu.Unlock()

	plan, err := compiler.Compile(context.Background(), LQLCompileRequest{
		Source: "from events | take 1",
		Target: "duckdb",
		Limit:  1,
	})
	if err != nil {
		t.Fatalf("compile after subprocess loss: %v", err)
	}
	if plan.SQL == "" {
		t.Fatal("restarted compiler returned an empty plan")
	}
	if compiler.cmd == nil || compiler.cmd.Process == nil || compiler.cmd.Process.Pid == previousPID {
		t.Fatalf("compiler process was not replaced: old pid=%d", previousPID)
	}
}

func TestLQLStdioCompilerBacksOffFailedRestarts(t *testing.T) {
	binary := testLQLBinary(t)
	compiler, err := newLQLStdioCompiler(context.Background(), testLQLConfig(binary))
	if err != nil {
		t.Fatal(err)
	}
	defer compiler.Close(context.Background())

	compiler.mu.Lock()
	_ = compiler.killLocked()
	compiler.binary = filepath.Join(t.TempDir(), "missing-lql")
	compiler.mu.Unlock()

	request := LQLCompileRequest{Source: "from events | take 1", Target: "duckdb", Limit: 1}
	if _, err := compiler.Compile(context.Background(), request); err == nil {
		t.Fatal("expected failed compiler restart")
	}

	compiler.mu.Lock()
	compiler.binary = binary
	compiler.mu.Unlock()
	if _, err := compiler.Compile(context.Background(), request); err == nil {
		t.Fatal("expected restart backoff after a failed launch")
	}

	time.Sleep(150 * time.Millisecond)
	if _, err := compiler.Compile(context.Background(), request); err != nil {
		t.Fatalf("compile after restart backoff: %v", err)
	}
}
