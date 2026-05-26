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
	SQL     string                   `json:"sql,omitempty"`
}

// QueryLQL sends a LQL query to the collector's /lql/query endpoint.
// The collector will compile LQL to SQL server-side and return results.
func (c *CollectorClient) QueryLQL(ctx context.Context, lql string) (*QueryResult, error) {
	body, err := json.Marshal(map[string]string{"query": lql})
	if err != nil {
		return nil, fmt.Errorf("lql: marshal: %w", err)
	}
	raw, err := c.do(ctx, "POST", "/lql/query", body)
	if err != nil {
		return nil, err
	}
	var result QueryResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("lql: decode: %w", err)
	}
	return &result, nil
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
