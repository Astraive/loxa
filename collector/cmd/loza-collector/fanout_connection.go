package main

import (
	"fmt"
	"strings"
)

func applyFanoutConnection(output *collectorFanoutOutput, cfg collectorConfig) error {
	name := strings.TrimSpace(output.connection)
	for _, connection := range cfg.databaseConnections {
		if connection.name != name {
			continue
		}
		backend := connection.backend
		outputType := output.sinkType
		if outputType == "" {
			outputType = fanoutSinkDuckDB
		}
		if outputType == "postgresql" {
			outputType = fanoutSinkPostgres
		}
		if backend != outputType {
			return fmt.Errorf("fanout output %q connection %q backend %q does not match output type %q", output.name, name, backend, outputType)
		}
		switch backend {
		case fanoutSinkDuckDB:
			output.duckDBPath = connection.path
			output.duckDBTable = connection.table
			output.duckDBRawColumn = connection.rawColumn
			output.duckDBStoreRaw = &connection.storeRaw
		case fanoutSinkPostgres:
			output.pgDSN = postgresDSN(connection)
			output.pgTable = connection.table
			output.pgRawColumn = connection.rawColumn
			output.pgStoreRaw = connection.storeRaw
			output.pgSchema = connection.schema
		case fanoutSinkClickHouse:
			output.chAddrs = append([]string(nil), connection.hosts...)
			output.chDatabase = connection.database
			output.chUsername = connection.username
			output.chPassword = connection.password
			output.chTable = connection.table
			output.chRawColumn = connection.rawColumn
			output.chStoreRaw = connection.storeRaw
			output.chSchema = connection.schema
		}
		return nil
	}
	return fmt.Errorf("fanout output %q references unknown database connection %q", output.name, name)
}
