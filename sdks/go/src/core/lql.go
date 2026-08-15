package core

import (
	"context"
	"encoding/json"
	"fmt"
)

// QueryResult holds the result of a query against the collector.
type QueryResult struct {
	Columns []string                 `json:"columns"`
	Rows    []map[string]interface{} `json:"rows"`
}

// QueryValue is a typed value supplied to an LQL query.
type QueryValue struct {
	Type  string `json:"type,omitempty"`
	Value any    `json:"value"`
}

// LQLQueryOptions controls server-side LQL compilation and execution.
type LQLQueryOptions struct {
	Parameters map[string]QueryValue
	Limit      int
}

// LQLDiagnostic is a structured compiler diagnostic.
type LQLDiagnostic struct {
	Code        string            `json:"code,omitempty"`
	Severity    string            `json:"severity,omitempty"`
	Message     string            `json:"message"`
	PrimarySpan json.RawMessage   `json:"primary_span,omitempty"`
	Labels      []json.RawMessage `json:"labels,omitempty"`
}

// LQLCompilationError is returned when the collector rejects LQL source.
type LQLCompilationError struct {
	Message     string          `json:"error,omitempty"`
	Diagnostics []LQLDiagnostic `json:"diagnostics,omitempty"`
	Status      int
}

func (e *LQLCompilationError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if len(e.Diagnostics) > 0 {
		return e.Diagnostics[0].Message
	}
	return "lql query failed"
}

type lqlQueryRequest struct {
	Query      string                 `json:"query"`
	Parameters map[string]QueryValue `json:"parameters"`
	Limit      int                    `json:"limit"`
}

// QueryLQL sends LQL source to /lql/query for server-side compilation.
// Optional options preserve compatibility with callers that only provide source.
func (c *CollectorClient) QueryLQL(ctx context.Context, lql string, options ...LQLQueryOptions) (*QueryResult, error) {
	var opts LQLQueryOptions
	if len(options) > 0 {
		opts = options[0]
	}
	if opts.Limit <= 0 {
		opts.Limit = 1000
	} else if opts.Limit > 1000 {
		opts.Limit = 1000
	}
	parameters := opts.Parameters
	if parameters == nil {
		parameters = map[string]QueryValue{}
	}
	body, err := json.Marshal(lqlQueryRequest{
		Query:      lql,
		Parameters: parameters,
		Limit:      opts.Limit,
	})
	if err != nil {
		return nil, fmt.Errorf("lql: marshal: %w", err)
	}
	raw, err := c.do(ctx, "POST", "/lql/query", body)
	if err != nil {
		var response struct {
			Error       string          `json:"error,omitempty"`
			Diagnostics []LQLDiagnostic `json:"diagnostics,omitempty"`
		}
		if json.Unmarshal(raw, &response) == nil && (response.Error != "" || len(response.Diagnostics) > 0) {
			return nil, &LQLCompilationError{
				Message:     response.Error,
				Diagnostics: response.Diagnostics,
			}
		}
		return nil, err
	}
	var result QueryResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("lql: decode: %w", err)
	}
	return &result, nil
}

// QueryLQLWithOptions is an explicit options-form alias for QueryLQL.
func (c *CollectorClient) QueryLQLWithOptions(ctx context.Context, lql string, options LQLQueryOptions) (*QueryResult, error) {
	return c.QueryLQL(ctx, lql, options)
}

// QuerySQL sends a raw SQL query to the collector and returns parsed results.
func (c *CollectorClient) QuerySQL(ctx context.Context, engine, sql string) (*QueryResult, error) {
	body, err := json.Marshal(map[string]string{"query": sql, "engine": engine})
	if err != nil {
		return nil, fmt.Errorf("query: marshal: %w", err)
	}
	raw, err := c.do(ctx, "POST", "/query", body)
	if err != nil {
		return nil, err
	}
	var result QueryResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("query: decode: %w", err)
	}
	return &result, nil
}
