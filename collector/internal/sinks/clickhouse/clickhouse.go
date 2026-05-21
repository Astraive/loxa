package clickhouse

import (
	"context"
	"crypto/tls"
	"fmt"
	"regexp"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	collectorevent "github.com/astraive/loxa-collector/internal/event"
	"github.com/astraive/loxa-collector/internal/sinks/internal/atrest"
	"github.com/astraive/loxa-collector/internal/sinks/internal/projection"
)

// Config controls ClickHouse sink behavior.
type Config struct {
	Addrs    []string
	Database string
	Username string
	Password string
	Table    string
	Schema   map[string]string
	// StoreRaw stores encoded event in RawColumn when schema mode is enabled.
	StoreRaw bool
	// RawColumn is used for raw mode and optional raw storage in schema mode.
	RawColumn string
	// TLSConfig enables TLS for the ClickHouse connection when provided.
	TLSConfig  *tls.Config
	EncryptRaw bool
	EncryptKey string
}

type sink struct {
	conn        driver.Conn
	table       string
	rawColumn   string
	schema      map[string]string
	columns     []string
	rawQuery    string
	schemaQuery string
	storeRaw    bool
	encryptRaw  bool
	encryptKey  string
}

var tableIdentPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// New creates a ClickHouse sink.
func New(cfg Config) (collectorevent.Sink, error) {
	if len(cfg.Addrs) == 0 {
		return nil, fmt.Errorf("clickhouse: Addrs is required")
	}
	if cfg.Database == "" {
		cfg.Database = "default"
	}
	if cfg.Table == "" {
		cfg.Table = "loxa_events"
	}
	if cfg.RawColumn == "" {
		cfg.RawColumn = "raw"
	}
	rawColumnName := strings.TrimSpace(cfg.RawColumn)
	quotedTable, err := quoteTableName(cfg.Table, "`")
	if err != nil {
		return nil, err
	}
	quotedRaw, err := quoteIdentifier(rawColumnName, "`")
	if err != nil {
		return nil, fmt.Errorf("clickhouse: invalid raw column: %w", err)
	}

	var schema map[string]string
	var columns []string
	var schemaQuery string
	storeRaw := false
	if len(cfg.Schema) > 0 {
		schema = make(map[string]string, len(cfg.Schema))
		for col, path := range cfg.Schema {
			col = strings.TrimSpace(col)
			if _, err := quoteIdentifier(col, "`"); err != nil {
				return nil, fmt.Errorf("clickhouse: invalid schema column %q", col)
			}
			path = strings.TrimSpace(path)
			if path == "" {
				return nil, fmt.Errorf("clickhouse: empty schema path for column %q", col)
			}
			if err := projection.ValidatePath(path); err != nil {
				return nil, fmt.Errorf("clickhouse: invalid schema path for column %q: %w", col, err)
			}
			schema[col] = path
		}
		if cfg.StoreRaw {
			if _, exists := schema[rawColumnName]; exists {
				return nil, fmt.Errorf("clickhouse: raw column %q conflicts with schema column", rawColumnName)
			}
		}
		columns = projection.SortedColumns(schema)
		storeRaw = cfg.StoreRaw
		schemaQuery = buildSchemaInsertQuery(quotedTable, columns, storeRaw, quotedRaw)
	}

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: cfg.Addrs,
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		TLS: cfg.TLSConfig,
	})
	if err != nil {
		return nil, err
	}
	return &sink{
		conn:        conn,
		table:       quotedTable,
		rawColumn:   quotedRaw,
		schema:      schema,
		columns:     columns,
		rawQuery:    fmt.Sprintf("INSERT INTO %s (%s) VALUES (?)", quotedTable, quotedRaw),
		schemaQuery: schemaQuery,
		storeRaw:    storeRaw,
		encryptRaw:  cfg.EncryptRaw,
		encryptKey:  cfg.EncryptKey,
	}, nil
}

func (s *sink) Name() string { return "clickhouse" }

func (s *sink) WriteEvent(ctx context.Context, encoded []byte, _ *collectorevent.Event) error {
	if len(s.columns) == 0 {
		value := string(encoded)
		if s.encryptRaw {
			enc, err := atrest.EncryptString(encoded, s.encryptKey)
			if err != nil {
				return err
			}
			value = enc
		}
		return s.conn.Exec(ctx, s.rawQuery, value)
	}

	values, err := projection.ProjectValues(encoded, s.schema, s.columns)
	if err != nil {
		return err
	}
	args := make([]any, 0, len(values)+1)
	args = append(args, values...)
	if s.storeRaw {
		if s.encryptRaw {
			enc, err := atrest.EncryptString(encoded, s.encryptKey)
			if err != nil {
				return err
			}
			args = append(args, enc)
		} else {
			args = append(args, string(encoded))
		}
	}
	return s.conn.Exec(ctx, s.schemaQuery, args...)
}

func (s *sink) Flush(_ context.Context) error { return nil }

func (s *sink) Close(_ context.Context) error { return s.conn.Close() }

func buildBatchInsertStmt(table string, columns []string, storeRaw bool, rawColumn string) string {
	cols := make([]string, 0, len(columns)+1)
	for _, c := range columns {
		cols = append(cols, "`"+c+"`")
	}
	if storeRaw {
		cols = append(cols, rawColumn)
	}
	return fmt.Sprintf("INSERT INTO %s (%s) VALUES", table, strings.Join(cols, ", "))
}

// WriteBatch writes multiple events efficiently using ClickHouse batch API.
func (s *sink) WriteBatch(ctx context.Context, events [][]byte) error {
	if len(events) == 0 {
		return nil
	}
	// Raw-only mode
	if len(s.columns) == 0 {
		stmt := buildBatchInsertStmt(s.table, nil, false, s.rawColumn)
		batch, err := s.conn.PrepareBatch(ctx, stmt)
		if err != nil {
			return err
		}
		for _, e := range events {
			val := string(e)
			if s.encryptRaw {
				enc, err := atrest.EncryptString(e, s.encryptKey)
				if err != nil {
					_ = batch.Abort()
					return err
				}
				val = enc
			}
			if err := batch.Append(val); err != nil {
				_ = batch.Abort()
				return err
			}
		}
		return batch.Send()
	}

	// Schema mode
	stmt := buildBatchInsertStmt(s.table, s.columns, s.storeRaw, s.rawColumn)
	batch, err := s.conn.PrepareBatch(ctx, stmt)
	if err != nil {
		return err
	}
	for _, e := range events {
		values, err := projection.ProjectValues(e, s.schema, s.columns)
		if err != nil {
			_ = batch.Abort()
			return err
		}
		args := make([]any, 0, len(values)+1)
		for _, v := range values {
			args = append(args, v)
		}
		if s.storeRaw {
			if s.encryptRaw {
				enc, err := atrest.EncryptString(e, s.encryptKey)
				if err != nil {
					_ = batch.Abort()
					return err
				}
				args = append(args, enc)
			} else {
				args = append(args, string(e))
			}
		}
		if err := batch.Append(args...); err != nil {
			_ = batch.Abort()
			return err
		}
	}
	return batch.Send()
}

func quoteTableName(table, quote string) (string, error) {
	table = strings.TrimSpace(table)
	if table == "" {
		return "", fmt.Errorf("clickhouse: table is required")
	}
	parts := strings.Split(table, ".")
	quoted := make([]string, 0, len(parts))
	for _, part := range parts {
		if !tableIdentPattern.MatchString(part) {
			return "", fmt.Errorf("clickhouse: invalid table identifier %q", table)
		}
		quoted = append(quoted, quote+part+quote)
	}
	return strings.Join(quoted, "."), nil
}

func quoteIdentifier(ident, quote string) (string, error) {
	ident = strings.TrimSpace(ident)
	if !tableIdentPattern.MatchString(ident) {
		return "", fmt.Errorf("invalid identifier %q", ident)
	}
	return quote + ident + quote, nil
}

func buildSchemaInsertQuery(table string, columns []string, storeRaw bool, rawColumn string) string {
	quotedCols := make([]string, 0, len(columns)+1)
	for _, c := range columns {
		quotedCols = append(quotedCols, "`"+c+"`")
	}
	if storeRaw {
		quotedCols = append(quotedCols, rawColumn)
	}
	placeholders := make([]string, len(quotedCols))
	for i := range placeholders {
		placeholders[i] = "?"
	}
	return fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		table,
		strings.Join(quotedCols, ", "),
		strings.Join(placeholders, ", "),
	)
}
