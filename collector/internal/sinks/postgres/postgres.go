package postgres

import (
	"context"
	"crypto/tls"
	"fmt"
	"regexp"
	"strings"

	collectorevent "github.com/astraive/loxa-collector/internal/event"
	"github.com/astraive/loxa-collector/internal/sinks/internal/atrest"
	"github.com/astraive/loxa-collector/internal/sinks/internal/projection"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config controls Postgres sink behavior.
type Config struct {
	DSN    string
	Table  string
	Schema map[string]string
	// StoreRaw stores the original encoded event in RawColumn in schema mode.
	StoreRaw bool
	// RawColumn is used for raw mode and optional raw storage in schema mode.
	RawColumn string
	// TLSConfig enables TLS for the Postgres connection when provided.
	TLSConfig  *tls.Config
	EncryptRaw bool
	EncryptKey string
}

type sink struct {
	pool        *pgxpool.Pool
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

// New creates a Postgres sink.
func New(ctx context.Context, cfg Config) (collectorevent.Sink, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("postgres: DSN is required")
	}
	if cfg.Table == "" {
		cfg.Table = "loxa_events"
	}
	if cfg.RawColumn == "" {
		cfg.RawColumn = "raw"
	}
	rawColumnName := strings.TrimSpace(cfg.RawColumn)
	quotedTable, err := quoteTableName(cfg.Table, `"`)
	if err != nil {
		return nil, err
	}
	quotedRaw, err := quoteIdentifier(rawColumnName, `"`)
	if err != nil {
		return nil, fmt.Errorf("postgres: invalid raw column: %w", err)
	}

	var schema map[string]string
	var columns []string
	var schemaQuery string
	storeRaw := false
	if len(cfg.Schema) > 0 {
		schema = make(map[string]string, len(cfg.Schema))
		for col, path := range cfg.Schema {
			col = strings.TrimSpace(col)
			if _, err := quoteIdentifier(col, `"`); err != nil {
				return nil, fmt.Errorf("postgres: invalid schema column %q", col)
			}
			path = strings.TrimSpace(path)
			if path == "" {
				return nil, fmt.Errorf("postgres: empty schema path for column %q", col)
			}
			if err := projection.ValidatePath(path); err != nil {
				return nil, fmt.Errorf("postgres: invalid schema path for column %q: %w", col, err)
			}
			schema[col] = path
		}
		if cfg.StoreRaw {
			if _, exists := schema[rawColumnName]; exists {
				return nil, fmt.Errorf("postgres: raw column %q conflicts with schema column", rawColumnName)
			}
		}
		columns = projection.SortedColumns(schema)
		storeRaw = cfg.StoreRaw
		schemaQuery = buildSchemaInsertQuery(quotedTable, columns, storeRaw, quotedRaw)
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, err
	}
	if cfg.TLSConfig != nil {
		poolCfg.ConnConfig.TLSConfig = cfg.TLSConfig.Clone()
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, err
	}

	return &sink{
		pool:        pool,
		table:       quotedTable,
		rawColumn:   quotedRaw,
		schema:      schema,
		columns:     columns,
		rawQuery:    fmt.Sprintf("INSERT INTO %s (%s) VALUES ($1)", quotedTable, quotedRaw),
		schemaQuery: schemaQuery,
		storeRaw:    storeRaw,
		encryptRaw:  cfg.EncryptRaw,
		encryptKey:  cfg.EncryptKey,
	}, nil
}

func (s *sink) Name() string { return "postgres" }

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
		_, err := s.pool.Exec(ctx, s.rawQuery, value)
		return err
	}

	values, err := projection.ProjectValues(encoded, s.schema, s.columns)
	if err != nil {
		return err
	}
	if s.storeRaw {
		if s.encryptRaw {
			enc, err := atrest.EncryptString(encoded, s.encryptKey)
			if err != nil {
				return err
			}
			values = append(values, enc)
		} else {
			values = append(values, string(encoded))
		}
	}
	_, err = s.pool.Exec(ctx, s.schemaQuery, values...)
	return err
}

func (s *sink) Flush(_ context.Context) error { return nil }

func (s *sink) Close(_ context.Context) error {
	s.pool.Close()
	return nil
}

func quoteTableName(table, quote string) (string, error) {
	table = strings.TrimSpace(table)
	if table == "" {
		return "", fmt.Errorf("postgres: table is required")
	}
	parts := strings.Split(table, ".")
	quoted := make([]string, 0, len(parts))
	for _, part := range parts {
		if !tableIdentPattern.MatchString(part) {
			return "", fmt.Errorf("postgres: invalid table identifier %q", table)
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
		quotedCols = append(quotedCols, `"`+c+`"`)
	}
	if storeRaw {
		quotedCols = append(quotedCols, rawColumn)
	}
	placeholders := make([]string, len(quotedCols))
	for i := range placeholders {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	return fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		table,
		strings.Join(quotedCols, ", "),
		strings.Join(placeholders, ", "),
	)
}
