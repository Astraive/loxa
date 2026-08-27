package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/astraive/loza/collector/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/marcboeker/go-duckdb"
)

func openNamedDatabaseConnections(ctx context.Context, cfg collectorConfig) (map[string]database.Connection, map[string]database.Metadata, error) {
	connections := make(map[string]database.Connection)
	metadata := make(map[string]database.Metadata)
	closeAll := func() {
		for _, connection := range connections {
			_ = connection.Close(context.Background())
		}
	}
	for _, item := range cfg.databaseConnections {
		if !item.enabled {
			continue
		}
		openCtx, cancel := database.BoundedContext(ctx, item.connectionTimeout)
		var connection database.Connection
		var err error
		switch item.backend {
		case "duckdb":
			connection, err = openNamedDuckDB(openCtx, cfg, item)
		case "postgres":
			connection, err = openNamedPostgres(openCtx, cfg, item)
		case "clickhouse":
			connection, err = openNamedClickHouse(openCtx, cfg, item)
		default:
			err = fmt.Errorf("unsupported database backend %q", item.backend)
		}
		cancel()
		if err != nil {
			closeAll()
			return nil, nil, fmt.Errorf("database connection %q: %w", item.name, err)
		}
		pingCtx, pingCancel := database.BoundedContext(ctx, item.connectionTimeout)
		err = connection.Ping(pingCtx)
		pingCancel()
		if err != nil {
			_ = connection.Close(context.Background())
			closeAll()
			return nil, nil, fmt.Errorf("database connection %q health check failed: %w", item.name, err)
		}
		info := connection.Metadata()
		info.Primary = item.name == cfg.storageConnection
		connections[item.name] = connection
		metadata[item.name] = info
	}
	return connections, metadata, nil
}

func openNamedDuckDB(ctx context.Context, cfg collectorConfig, item databaseConnectionConfig) (database.Connection, error) {
	db, err := sql.Open(item.driver, item.path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(cfg.duckDBMaxOpenConns)
	db.SetMaxIdleConns(cfg.duckDBMaxIdleConns)
	if err := ensureSchema(db, databaseSchemaConfig(cfg, item)); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &database.SQL{DB: db, Info: database.Metadata{
		Name: item.name, Backend: "duckdb", Path: item.path, Database: item.database,
		Table: item.table, Enabled: item.enabled,
		Capabilities: []string{"query", "health", "write"},
	}}, nil
}

func openNamedPostgres(ctx context.Context, cfg collectorConfig, item databaseConnectionConfig) (database.Connection, error) {
	dsn := postgresDSN(item)
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	poolCfg.MaxConnLifetime = 30 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, err
	}
	return &database.Postgres{Pool: pool, Info: database.Metadata{
		Name: item.name, Backend: "postgres", Host: item.host, Port: item.port,
		Database: item.database, Table: item.table, Enabled: item.enabled,
		Capabilities: []string{"query", "health", "write"},
	}}, nil
}

func openNamedClickHouse(ctx context.Context, cfg collectorConfig, item databaseConnectionConfig) (database.Connection, error) {
	options := &clickhouse.Options{
		Addr: item.hosts,
		Auth: clickhouse.Auth{Database: item.database, Username: item.username, Password: item.password},
	}
	if item.tls {
		options.TLS = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	conn, err := clickhouse.Open(options)
	if err != nil {
		return nil, err
	}
	return &database.ClickHouse{Conn: conn, Info: database.Metadata{
		Name: item.name, Backend: "clickhouse", Host: strings.Join(item.hosts, ","),
		Database: item.database, Table: item.table, Enabled: item.enabled,
		Capabilities: []string{"query", "health", "write"},
	}}, nil
}

func postgresDSN(item databaseConnectionConfig) string {
	u := &url.URL{Scheme: "postgres", Host: net.JoinHostPort(item.host, strconv.Itoa(item.port)), Path: "/" + item.database}
	u.User = url.UserPassword(item.username, item.password)
	query := u.Query()
	sslMode := item.sslMode
	if sslMode == "" {
		sslMode = "require"
	}
	query.Set("sslmode", sslMode)
	u.RawQuery = query.Encode()
	return u.String()
}

func databaseSchemaConfig(cfg collectorConfig, item databaseConnectionConfig) collectorConfig {
	copy := cfg
	copy.duckDBPath = item.path
	copy.duckDBDriver = item.driver
	copy.duckDBTable = item.table
	copy.duckDBRawColumn = item.rawColumn
	copy.duckDBStoreRaw = item.storeRaw
	copy.duckDBSchema = item.schema
	copy.duckDBColumnTypes = item.columnTypes
	return copy
}
