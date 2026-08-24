package duckdb

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"sync"
	"time"

	collectorevent "github.com/astraive/loza/collector/internal/event"
	"github.com/astraive/loza/collector/internal/sinks/internal/atrest"
	"github.com/astraive/loza/collector/internal/sinks/internal/projection"
	"github.com/marcboeker/go-duckdb"
)

var identPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// DecryptRaw decodes a raw event written with Config.EncryptRaw.
func DecryptRaw(data []byte, key string) ([]byte, error) {
	return atrest.DecryptBytes(data, key)
}

func retrySleep(base time.Duration) time.Duration {
	if base <= 0 {
		return 0
	}
	return base + time.Duration(rand.Int63n(int64(base/2)))
}

// Config controls DuckDB sink behavior.
type Config struct {
	// DB can be supplied by caller. If nil, New opens a DB using Driver and DSN.
	DB *sql.DB
	// Driver defaults to "duckdb" when opening a new connection.
	Driver string
	// DSN is required when DB is nil.
	DSN string

	Table     string
	Schema    map[string]string
	StoreRaw  bool
	RawColumn string
	// BatchSize controls how many events are grouped into a transaction write.
	// Zero or one preserves per-event insert behavior.
	BatchSize int
	// FlushInterval periodically flushes buffered events when batching is enabled.
	// Zero disables periodic flushing.
	FlushInterval time.Duration
	// WriterLoop enables a single-writer queue with prepared-statement reuse.
	// This is optional and disabled by default.
	WriterLoop bool
	// WriterQueueSize controls queue capacity when WriterLoop is enabled.
	// If <= 0, a default is derived from BatchSize.
	WriterQueueSize int
	// WriteTimeout bounds internal write/flush operations that don't carry a caller deadline.
	// Zero means 10s.
	WriteTimeout time.Duration
	// RetryAttempts controls retries for retryable write failures.
	// Zero means no retries.
	RetryAttempts int
	// RetryBackoff controls delay between retries for retryable failures.
	// Zero means 50ms.
	RetryBackoff time.Duration
	// UseAppender enables DuckDB's Appender API for high-performance ingest.
	// Only effective when WriterLoop is enabled.
	UseAppender bool
	EncryptRaw  bool
	EncryptKey  string
}

type sink struct {
	db           *sql.DB
	ownsDB       bool
	table        string
	rawColumn    string
	schema       map[string]string
	columns      []string
	rawQuery     string
	schemaQuery  string
	insertQuery  string
	storeRaw     bool
	batchSize    int
	flushEvery   time.Duration
	batchEnabled bool

	writerLoop    bool
	useAppender   bool
	encryptRaw    bool
	encryptKey    string
	writeTimeout  time.Duration
	retryAttempts int
	retryBackoff  time.Duration
	writerStmt    *sql.Stmt

	mu      sync.Mutex
	flushMu sync.Mutex
	buffer  [][]any
	closed  bool
	lastErr error
	stopCh  chan struct{}
	stopWG  sync.WaitGroup

	writerCh chan []any
	flushCh  chan chan error
}

// New creates a DuckDB sink.
func New(cfg Config) (collectorevent.Sink, error) {
	if cfg.EncryptRaw && strings.TrimSpace(cfg.EncryptKey) == "" {
		return nil, errors.New("duckdb: encrypt_key must not be empty when encrypt_raw is enabled")
	}
	if cfg.Table == "" {
		cfg.Table = "loza_events"
	}
	if cfg.RawColumn == "" {
		cfg.RawColumn = "raw"
	}
	rawColumnName := strings.TrimSpace(cfg.RawColumn)
	quotedTable, err := quoteTableName(cfg.Table)
	if err != nil {
		return nil, err
	}
	quotedRaw, err := quoteIdentifier(rawColumnName)
	if err != nil {
		return nil, fmt.Errorf("duckdb: invalid raw column: %w", err)
	}

	var schema map[string]string
	var columns []string
	var schemaQuery string
	storeRaw := false
	if len(cfg.Schema) > 0 {
		schema = make(map[string]string, len(cfg.Schema))
		for col, path := range cfg.Schema {
			col = strings.TrimSpace(col)
			if _, err := quoteIdentifier(col); err != nil {
				return nil, fmt.Errorf("duckdb: invalid schema column %q", col)
			}
			path = strings.TrimSpace(path)
			if path == "" {
				return nil, fmt.Errorf("duckdb: empty schema path for column %q", col)
			}
			if err := projection.ValidatePath(path); err != nil {
				return nil, fmt.Errorf("duckdb: invalid schema path for column %q: %w", col, err)
			}
			schema[col] = path
		}
		if cfg.StoreRaw {
			if _, exists := schema[rawColumnName]; exists {
				return nil, fmt.Errorf("duckdb: raw column %q conflicts with schema column", rawColumnName)
			}
		}
		columns = projection.SortedColumns(schema)
		storeRaw = cfg.StoreRaw
		schemaQuery = buildSchemaInsertQuery(quotedTable, columns, storeRaw, quotedRaw)
	}

	db := cfg.DB
	owns := false
	if db == nil {
		driver := cfg.Driver
		if driver == "" {
			driver = "duckdb"
		}
		if strings.TrimSpace(cfg.DSN) == "" {
			return nil, fmt.Errorf("duckdb: DSN is required when DB is nil")
		}
		db, err = sql.Open(driver, cfg.DSN)
		if err != nil {
			return nil, err
		}
		owns = true
	}

	s := &sink{
		db:            db,
		ownsDB:        owns,
		table:         quotedTable,
		rawColumn:     quotedRaw,
		schema:        schema,
		columns:       columns,
		rawQuery:      fmt.Sprintf("INSERT INTO %s (%s) VALUES (?)", quotedTable, quotedRaw),
		schemaQuery:   schemaQuery,
		storeRaw:      storeRaw,
		batchSize:     cfg.BatchSize,
		flushEvery:    cfg.FlushInterval,
		stopCh:        make(chan struct{}),
		writerLoop:    cfg.WriterLoop,
		useAppender:   cfg.UseAppender,
		encryptRaw:    cfg.EncryptRaw,
		encryptKey:    cfg.EncryptKey,
		writeTimeout:  cfg.WriteTimeout,
		retryAttempts: cfg.RetryAttempts,
		retryBackoff:  cfg.RetryBackoff,
	}
	if s.writeTimeout <= 0 {
		s.writeTimeout = 10 * time.Second
	}
	if s.retryAttempts < 0 {
		s.retryAttempts = 0
	}
	if s.retryBackoff <= 0 {
		s.retryBackoff = 50 * time.Millisecond
	}
	if s.batchSize <= 0 {
		s.batchSize = 1
	}
	s.insertQuery = s.rawQuery
	if len(s.columns) > 0 {
		s.insertQuery = s.schemaQuery
	}
	s.batchEnabled = s.batchSize > 1 || s.flushEvery > 0
	if s.writerLoop {
		queueSize := cfg.WriterQueueSize
		if queueSize <= 0 {
			queueSize = s.batchSize * 8
			if queueSize < 1024 {
				queueSize = 1024
			}
		}
		s.writerCh = make(chan []any, queueSize)
		s.flushCh = make(chan chan error)
		s.stopWG.Add(1)
		go s.writerWorker()
	} else if s.batchEnabled && s.flushEvery > 0 {
		s.stopWG.Add(1)
		go s.periodicFlush()
	}
	return s, nil
}

func (s *sink) Name() string { return "duckdb" }

func (s *sink) WriteEvent(ctx context.Context, encoded []byte, _ *collectorevent.Event) error {
	args, err := s.buildArgs(encoded)
	if err != nil {
		return err
	}

	if !s.batchEnabled {
		return s.execDirect(ctx, args)
	}

	if s.writerLoop {
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return errors.New("duckdb sink closed")
		}
		pendingErr := s.lastErr
		s.lastErr = nil
		s.mu.Unlock()
		if pendingErr != nil {
			return pendingErr
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.stopCh:
			return errors.New("duckdb sink closed")
		case s.writerCh <- cloneArgs(args):
			return nil
		}
	}

	var batch [][]any
	var pendingErr error
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return errors.New("duckdb sink closed")
	}
	if s.lastErr != nil {
		pendingErr = s.lastErr
		s.lastErr = nil
	}
	s.buffer = append(s.buffer, cloneArgs(args))
	if len(s.buffer) >= s.batchSize {
		batch = s.buffer
		s.buffer = nil
	}
	s.mu.Unlock()

	if pendingErr != nil {
		return pendingErr
	}
	if len(batch) == 0 {
		return nil
	}
	s.flushMu.Lock()
	defer s.flushMu.Unlock()
	return s.execBatch(ctx, batch)
}

func (s *sink) Flush(ctx context.Context) error {
	if s.writerLoop {
		s.mu.Lock()
		if s.closed {
			pendingErr := s.lastErr
			s.lastErr = nil
			s.mu.Unlock()
			return pendingErr
		}
		s.mu.Unlock()

		ack := make(chan error, 1)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.stopCh:
			return errors.New("duckdb sink closed")
		case s.flushCh <- ack:
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-ack:
			return err
		}
	}
	s.flushMu.Lock()
	defer s.flushMu.Unlock()

	s.mu.Lock()
	batch := s.buffer
	s.buffer = nil
	pendingErr := s.lastErr
	s.lastErr = nil
	s.mu.Unlock()

	if pendingErr != nil {
		return pendingErr
	}
	if len(batch) == 0 {
		return nil
	}
	return s.execBatch(ctx, batch)
}

func (s *sink) Close(ctx context.Context) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()

	close(s.stopCh)
	s.stopWG.Wait()

	var last error
	if err := s.Flush(ctx); err != nil {
		last = err
	}
	if s.ownsDB && s.db != nil {
		if err := s.db.Close(); err != nil {
			last = err
		}
	}
	return last
}

func (s *sink) periodicFlush() {
	defer s.stopWG.Done()
	t := time.NewTicker(s.flushEvery)
	defer t.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-t.C:
			ctx, cancel := context.WithTimeout(context.Background(), s.writeTimeout)
			err := s.Flush(ctx)
			cancel()
			if err != nil {
				s.mu.Lock()
				s.lastErr = err
				s.mu.Unlock()
			}
		}
	}
}

func (s *sink) writerWorker() {
	defer s.stopWG.Done()

	if s.useAppender {
		s.writerWorkerAppender()
		return
	}

	if stmt, err := s.db.Prepare(s.insertQuery); err != nil {
		s.mu.Lock()
		s.lastErr = err
		s.mu.Unlock()
		return
	} else {
		s.writerStmt = stmt
		defer stmt.Close()
	}

	var ticker *time.Ticker
	if s.flushEvery > 0 {
		ticker = time.NewTicker(s.flushEvery)
		defer ticker.Stop()
	}

	batch := make([][]any, 0, s.batchSize)
	flushBatch := func() error {
		if len(batch) == 0 {
			return nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), s.writeTimeout)
		err := s.execBatchPrepared(ctx, s.writerStmt, batch)
		cancel()
		batch = batch[:0]
		if err != nil {
			s.mu.Lock()
			s.lastErr = err
			s.mu.Unlock()
		}
		return err
	}

	for {
		var tick <-chan time.Time
		if ticker != nil {
			tick = ticker.C
		}
		select {
		case <-s.stopCh:
			for {
				select {
				case args := <-s.writerCh:
					batch = append(batch, args)
				default:
					_ = flushBatch()
					return
				}
			}
		case args := <-s.writerCh:
			batch = append(batch, args)
			if len(batch) >= s.batchSize {
				_ = flushBatch()
			}
		case ack := <-s.flushCh:
			ack <- flushBatch()
		case <-tick:
			_ = flushBatch()
		}
	}
}

func (s *sink) writerWorkerAppender() {
	// We need a dedicated connection for the Appender.
	conn, err := s.db.Conn(context.Background())
	if err != nil {
		s.mu.Lock()
		s.lastErr = err
		s.mu.Unlock()
		return
	}
	defer conn.Close()

	var appender *duckdb.Appender
	err = conn.Raw(func(driverConn any) error {
		dxc, ok := driverConn.(driver.Conn)
		if !ok {
			return fmt.Errorf("duckdb: not a driver.Conn")
		}
		// Extract table and schema from the quoted table name.
		// NewAppenderFromConn expects unquoted names.
		table := strings.ReplaceAll(s.table, `"`, "")
		schema := ""
		if parts := strings.Split(table, "."); len(parts) > 1 {
			schema = parts[0]
			table = parts[1]
		}
		var err error
		appender, err = duckdb.NewAppenderFromConn(dxc, schema, table)
		return err
	})

	if err != nil {
		s.mu.Lock()
		s.lastErr = err
		s.mu.Unlock()
		return
	}
	defer appender.Close()

	var ticker *time.Ticker
	if s.flushEvery > 0 {
		ticker = time.NewTicker(s.flushEvery)
		defer ticker.Stop()
	}

	count := 0
	flushBatch := func() error {
		if count == 0 {
			return nil
		}
		err := appender.Flush()
		count = 0
		if err != nil {
			s.mu.Lock()
			s.lastErr = err
			s.mu.Unlock()
		}
		return err
	}

	for {
		var tick <-chan time.Time
		if ticker != nil {
			tick = ticker.C
		}
		select {
		case <-s.stopCh:
			for {
				select {
				case args := <-s.writerCh:
					dargs := make([]driver.Value, len(args))
					for i, v := range args {
						dargs[i] = v
					}
					_ = appender.AppendRow(dargs...)
					count++
				default:
					_ = flushBatch()
					return
				}
			}
		case args := <-s.writerCh:
			dargs := make([]driver.Value, len(args))
			for i, v := range args {
				dargs[i] = v
			}
			if err := appender.AppendRow(dargs...); err != nil {
				s.mu.Lock()
				s.lastErr = err
				s.mu.Unlock()
			} else {
				count++
				if count >= s.batchSize {
					_ = flushBatch()
				}
			}
		case ack := <-s.flushCh:
			ack <- flushBatch()
		case <-tick:
			_ = flushBatch()
		}
	}
}

func (s *sink) buildArgs(encoded []byte) ([]any, error) {
	if len(s.columns) == 0 {
		if s.encryptRaw {
			enc, err := atrest.EncryptString(encoded, s.encryptKey)
			if err != nil {
				return nil, err
			}
			return []any{enc}, nil
		}
		return []any{string(encoded)}, nil
	}

	values, err := projection.ProjectValues(encoded, s.schema, s.columns)
	if err != nil {
		return nil, err
	}
	args := make([]any, 0, len(values)+1)
	args = append(args, values...)
	if s.storeRaw {
		if s.encryptRaw {
			enc, err := atrest.EncryptString(encoded, s.encryptKey)
			if err != nil {
				return nil, err
			}
			args = append(args, enc)
		} else {
			args = append(args, string(encoded))
		}
	}
	return args, nil
}

func (s *sink) execDirect(ctx context.Context, args []any) error {
	ctx, cancel := s.contextWithWriteTimeout(ctx)
	defer cancel()
	var last error
	for attempt := 0; attempt <= s.retryAttempts; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		_, err := s.db.ExecContext(ctx, s.insertQuery, args...)
		if err == nil {
			return nil
		}
		last = err
		if !isRetryableError(err) || attempt == s.retryAttempts {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retrySleep(s.retryBackoff)):
		}
	}
	return last
}

func (s *sink) execBatch(ctx context.Context, batch [][]any) error {
	ctx, cancel := s.contextWithWriteTimeout(ctx)
	defer cancel()
	var last error
	for attempt := 0; attempt <= s.retryAttempts; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err := s.execBatchOnce(ctx, batch)
		if err == nil {
			return nil
		}
		last = err
		if !isRetryableError(err) || attempt == s.retryAttempts {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retrySleep(s.retryBackoff)):
		}
	}
	return last
}

func (s *sink) execBatchOnce(ctx context.Context, batch [][]any) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	stmt, err := tx.PrepareContext(ctx, s.insertQuery)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	for _, args := range batch {
		if _, err := stmt.ExecContext(ctx, args...); err != nil {
			_ = stmt.Close()
			_ = tx.Rollback()
			tx = nil
			return err
		}
	}

	if err := stmt.Close(); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *sink) execBatchPrepared(ctx context.Context, stmt *sql.Stmt, batch [][]any) error {
	ctx, cancel := s.contextWithWriteTimeout(ctx)
	defer cancel()
	var last error
	for attempt := 0; attempt <= s.retryAttempts; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err := s.execBatchPreparedOnce(ctx, stmt, batch)
		if err == nil {
			return nil
		}
		last = err
		if !isRetryableError(err) || attempt == s.retryAttempts {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retrySleep(s.retryBackoff)):
		}
	}
	return last
}

func (s *sink) execBatchPreparedOnce(ctx context.Context, stmt *sql.Stmt, batch [][]any) error {
	for _, args := range batch {
		if _, err := stmt.ExecContext(ctx, args...); err != nil {
			return err
		}
	}
	return nil
}

func (s *sink) contextWithWriteTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		return context.WithTimeout(context.Background(), s.writeTimeout)
	}
	if _, hasDeadline := ctx.Deadline(); hasDeadline || s.writeTimeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, s.writeTimeout)
}

func cloneArgs(args []any) []any {
	out := make([]any, len(args))
	copy(out, args)
	return out
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "busy") ||
		strings.Contains(msg, "locked") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "temporar")
}

func quoteTableName(table string) (string, error) {
	table = strings.TrimSpace(table)
	if table == "" {
		return "", fmt.Errorf("duckdb: table is required")
	}
	parts := strings.Split(table, ".")
	quoted := make([]string, 0, len(parts))
	for _, part := range parts {
		if !identPattern.MatchString(part) {
			return "", fmt.Errorf("duckdb: invalid table identifier %q", table)
		}
		quoted = append(quoted, `"`+part+`"`)
	}
	return strings.Join(quoted, "."), nil
}

func quoteIdentifier(ident string) (string, error) {
	ident = strings.TrimSpace(ident)
	if !identPattern.MatchString(ident) {
		return "", fmt.Errorf("invalid identifier %q", ident)
	}
	return `"` + ident + `"`, nil
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
		placeholders[i] = "?"
	}
	return fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		table,
		strings.Join(quotedCols, ", "),
		strings.Join(placeholders, ", "),
	)
}
