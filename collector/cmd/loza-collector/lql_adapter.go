package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type lqlStdioCompiler struct {
	mu               sync.Mutex
	cmd              *exec.Cmd
	stdin            io.WriteCloser
	stdout           *bufio.Reader
	binary           string
	protocol         int
	compiler         string
	language         string
	timeout          time.Duration
	startupTimeout   time.Duration
	sem              chan struct{}
	nextID           int64
	restartFailures  uint
	restartNotBefore time.Time
	lastRestartError error
}

func newLQLStdioCompiler(ctx context.Context, cfg collectorConfig) (*lqlStdioCompiler, error) {
	if strings.TrimSpace(cfg.lqlBinary) == "" {
		return nil, errors.New("lql compiler binary is empty")
	}
	startup := cfg.lqlStartupTimeout
	if startup <= 0 {
		startup = 5 * time.Second
	}
	compileTimeout := cfg.lqlCompileTimeout
	if compileTimeout <= 0 {
		compileTimeout = 5 * time.Second
	}
	maxConcurrent := cfg.lqlMaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 8
	}
	compiler := &lqlStdioCompiler{
		binary: cfg.lqlBinary, protocol: cfg.lqlExpectedProtocol,
		compiler: cfg.lqlExpectedCompiler, language: cfg.lqlExpectedLanguage,
		timeout: compileTimeout, startupTimeout: startup, sem: make(chan struct{}, maxConcurrent),
	}
	if compiler.protocol == 0 {
		compiler.protocol = 1
	}
	if compiler.language == "" {
		compiler.language = "0.1"
	}
	startCtx, cancel := context.WithTimeout(ctx, startup)
	defer cancel()
	if err := compiler.start(startCtx); err != nil {
		return nil, err
	}
	return compiler, nil
}

func (c *lqlStdioCompiler) start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.startLocked(ctx)
}

func (c *lqlStdioCompiler) startLocked(ctx context.Context) error {
	cmd := exec.Command(c.binary, "serve", "--stdio")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return err
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return err
	}
	c.cmd, c.stdin, c.stdout = cmd, stdin, bufio.NewReader(stdout)
	response, err := c.requestLocked(ctx, "handshake", map[string]any{})
	if err != nil {
		_ = c.killLocked()
		return fmt.Errorf("lql handshake failed: %w", err)
	}
	var result struct {
		CompilerVersion string `json:"compiler_version"`
		LanguageVersion string `json:"language_version"`
		ProtocolVersion int    `json:"protocol_version"`
	}
	if err := json.Unmarshal(response.Result, &result); err != nil {
		_ = c.killLocked()
		return fmt.Errorf("invalid lql handshake: %w", err)
	}
	if result.ProtocolVersion != c.protocol || (c.compiler != "" && result.CompilerVersion != c.compiler) || result.LanguageVersion != c.language {
		_ = c.killLocked()
		return fmt.Errorf("lql version mismatch: protocol=%d compiler=%s language=%s", result.ProtocolVersion, result.CompilerVersion, result.LanguageVersion)
	}
	c.restartFailures = 0
	c.restartNotBefore = time.Time{}
	c.lastRestartError = nil
	return nil
}

func (c *lqlStdioCompiler) Compile(ctx context.Context, req LQLCompileRequest) (ParameterizedPlan, error) {
	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Done():
		return ParameterizedPlan{}, ctx.Err()
	}
	if req.Limit <= 0 {
		req.Limit = 1000
	}
	if req.Limit > 1000 {
		req.Limit = 1000
	}
	source := strings.TrimSpace(req.Source)
	if source == "" {
		return ParameterizedPlan{}, NewLQLCompileError(LQLDiagnostic{Code: "LQL000", Severity: "error", Message: "query is empty"})
	}
	deadline := c.timeout
	if deadline <= 0 {
		deadline = 5 * time.Second
	}
	compileCtx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.ensureStartedLocked(compileCtx); err != nil {
		return ParameterizedPlan{}, NewLQLCompilerUnavailable(err)
	}
	response, err := c.requestLocked(compileCtx, "compile", map[string]any{
		"query":      source,
		"target":     req.Target,
		"parameters": req.Parameters,
		"schema":     nil,
		"policy": map[string]any{
			"allowed_sources": []string{"events"},
			"max_result_rows": req.Limit,
		},
		"scope": req.Scope,
	})
	if err != nil {
		_ = c.killLocked()
		c.restartFailures = 0
		c.restartNotBefore = time.Time{}
		c.lastRestartError = err
		return ParameterizedPlan{}, NewLQLCompilerUnavailable(err)
	}
	if response.Error != nil {
		if len(response.Error.Diagnostics) > 0 {
			return ParameterizedPlan{}, &LQLCompileError{Diagnostics: response.Error.Diagnostics}
		}
		return ParameterizedPlan{}, NewLQLCompilerUnavailable(errors.New(response.Error.Message))
	}
	var body struct {
		Plan struct {
			SQL        string `json:"sql"`
			Parameters []struct {
				LogicalType string `json:"logical_type"`
				Value       any    `json:"value"`
			} `json:"parameters"`
			OutputSchema []LQLColumn `json:"output_schema"`
		} `json:"plan"`
	}
	if err := json.Unmarshal(response.Result, &body); err != nil {
		_ = c.killLocked()
		c.restartFailures = 0
		c.restartNotBefore = time.Time{}
		c.lastRestartError = err
		return ParameterizedPlan{}, NewLQLCompilerUnavailable(err)
	}
	for index := range body.Plan.OutputSchema {
		if body.Plan.OutputSchema[index].Type == "" {
			body.Plan.OutputSchema[index].Type = body.Plan.OutputSchema[index].FieldType
		}
	}
	args := make([]any, 0, len(body.Plan.Parameters))
	for _, parameter := range body.Plan.Parameters {
		args = append(args, parameter.Value)
	}
	return ParameterizedPlan{SQL: body.Plan.SQL, Args: args, OutputSchema: body.Plan.OutputSchema}, nil
}

func (c *lqlStdioCompiler) Close(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.killLocked()
}

func (c *lqlStdioCompiler) killLocked() error {
	stdin, cmd := c.stdin, c.cmd
	c.cmd, c.stdin, c.stdout = nil, nil, nil
	if stdin != nil {
		_ = stdin.Close()
	}
	if cmd == nil {
		return nil
	}
	_ = cmd.Process.Kill()
	return cmd.Wait()
}

func (c *lqlStdioCompiler) ensureStartedLocked(ctx context.Context) error {
	if c.cmd != nil && c.stdin != nil && c.stdout != nil {
		return nil
	}
	if wait := time.Until(c.restartNotBefore); wait > 0 {
		return fmt.Errorf("lql compiler restart backoff active: %w", c.lastRestartError)
	}
	startCtx := ctx
	cancel := func() {}
	if c.startupTimeout > 0 {
		startCtx, cancel = context.WithTimeout(ctx, c.startupTimeout)
	}
	defer cancel()
	if err := c.startLocked(startCtx); err != nil {
		c.restartFailures++
		c.lastRestartError = err
		c.restartNotBefore = time.Now().Add(lqlRestartBackoff(c.restartFailures))
		return err
	}
	return nil
}

func lqlRestartBackoff(failures uint) time.Duration {
	delay := 100 * time.Millisecond
	for attempt := uint(1); attempt < failures && delay < 5*time.Second; attempt++ {
		delay *= 2
	}
	if delay > 5*time.Second {
		return 5 * time.Second
	}
	return delay
}

type lqlRPCResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code        string          `json:"code"`
		Message     string          `json:"message"`
		Diagnostics []LQLDiagnostic `json:"diagnostics"`
	} `json:"error"`
}

func (c *lqlStdioCompiler) requestLocked(ctx context.Context, method string, params any) (lqlRPCResponse, error) {
	c.nextID++
	request := map[string]any{"id": c.nextID, "method": method, "protocol_version": c.protocol, "language_version": c.language, "params": params}
	line, err := json.Marshal(request)
	if err != nil {
		return lqlRPCResponse{}, err
	}
	if _, err := c.stdin.Write(append(line, '\n')); err != nil {
		return lqlRPCResponse{}, err
	}
	result := make(chan lqlRPCResponse, 1)
	errorsCh := make(chan error, 1)
	go func() {
		responseLine, readErr := c.stdout.ReadBytes('\n')
		if readErr != nil {
			errorsCh <- readErr
			return
		}
		var response lqlRPCResponse
		errorsCh <- json.Unmarshal(responseLine, &response)
		result <- response
	}()
	select {
	case <-ctx.Done():
		return lqlRPCResponse{}, ctx.Err()
	case err := <-errorsCh:
		if err != nil {
			return lqlRPCResponse{}, err
		}
		return <-result, nil
	}
}
func escapeLQLString(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, `\`, `\\`), `"`, `\"`)
}
