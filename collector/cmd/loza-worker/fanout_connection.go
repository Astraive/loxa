package main

import (
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	collectorconfig "github.com/astraive/loza/collector/internal/config"
)

func fanoutOutputsFromFileWithConnections(outputs []collectorconfig.FanoutOutputConfig, connections []collectorconfig.DatabaseConnectionConfig) []workerFanoutOutput {
	mapped := fanoutOutputsFromFile(outputs)
	byName := make(map[string]collectorconfig.DatabaseConnectionConfig, len(connections))
	for _, connection := range connections {
		byName[strings.TrimSpace(connection.Name)] = connection
	}
	for i := range mapped {
		name := mapped[i].connection
		if name == "" {
			continue
		}
		connection, ok := byName[name]
		if !ok {
			continue
		}
		backend := strings.ToLower(strings.TrimSpace(connection.Type))
		if backend == "postgresql" {
			backend = fanoutSinkPostgres
		}
		switch backend {
		case fanoutSinkDuckDB:
			mapped[i].duckDBPath = strings.TrimSpace(connection.Path)
			mapped[i].duckDBTable = strings.TrimSpace(connection.Table)
			mapped[i].duckDBRawColumn = strings.TrimSpace(connection.RawColumn)
			mapped[i].duckDBStoreRaw = &connection.StoreRaw
		case fanoutSinkPostgres:
			mapped[i].pgDSN = workerPostgresDSN(connection)
			mapped[i].pgTable = strings.TrimSpace(connection.Table)
			mapped[i].pgRawColumn = strings.TrimSpace(connection.RawColumn)
			mapped[i].pgStoreRaw = connection.StoreRaw
			mapped[i].pgSchema = connection.Schema
		case fanoutSinkClickHouse:
			mapped[i].chAddrs = append([]string(nil), connection.Hosts...)
			mapped[i].chDatabase = strings.TrimSpace(connection.Database)
			mapped[i].chUsername = os.Getenv(strings.TrimSpace(connection.UsernameEnv))
			mapped[i].chPassword = os.Getenv(strings.TrimSpace(connection.PasswordEnv))
			mapped[i].chTable = strings.TrimSpace(connection.Table)
			mapped[i].chRawColumn = strings.TrimSpace(connection.RawColumn)
			mapped[i].chStoreRaw = connection.StoreRaw
			mapped[i].chSchema = connection.Schema
		}
	}
	return mapped
}

func workerPostgresDSN(connection collectorconfig.DatabaseConnectionConfig) string {
	port := connection.Port
	if port == 0 {
		port = 5432
	}
	u := &url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(strings.TrimSpace(connection.Host), strconv.Itoa(port)),
		Path:   "/" + strings.TrimSpace(connection.Database),
		User:   url.UserPassword(os.Getenv(connection.UsernameEnv), os.Getenv(connection.PasswordEnv)),
	}
	mode := strings.TrimSpace(connection.SSLMode)
	if mode == "" {
		mode = "require"
	}
	query := u.Query()
	query.Set("sslmode", mode)
	u.RawQuery = query.Encode()
	return u.String()
}
