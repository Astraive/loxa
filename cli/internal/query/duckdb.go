package query

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/marcboeker/go-duckdb"
)

func RunDuckDB(dbPath, sqlQuery string) error {
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	// Disable external access to block read_csv, read_json, etc.
	if _, err := db.Exec("SET enable_external_access=false"); err != nil {
		return fmt.Errorf("set safety guard: %w", err)
	}

	rows, err := db.Query(sqlQuery)
	if err != nil {
		return fmt.Errorf("execute query: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("get columns: %w", err)
	}
	fmt.Println(strings.Join(columns, "\t"))

	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return fmt.Errorf("scan row: %w", err)
		}
		for i, value := range values {
			if i > 0 {
				fmt.Print("\t")
			}
			fmt.Print(fmt.Sprint(value))
		}
		fmt.Println()
	}

	return nil
}
