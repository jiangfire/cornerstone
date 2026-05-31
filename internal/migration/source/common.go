package source

import (
	"database/sql"
	"fmt"
	"strings"
)

func scanRows(rows *sql.Rows) ([]map[string]interface{}, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("read row columns: %w", err)
	}

	result := make([]map[string]interface{}, 0)
	for rows.Next() {
		values := make([]interface{}, len(columns))
		scanTargets := make([]interface{}, len(columns))
		for idx := range values {
			scanTargets[idx] = &values[idx]
		}
		if err := rows.Scan(scanTargets...); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		row := make(map[string]interface{}, len(columns))
		for idx, column := range columns {
			row[column] = normalizeSQLValue(values[idx])
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func normalizeSQLValue(value interface{}) interface{} {
	switch v := value.(type) {
	case []byte:
		return string(v)
	default:
		return v
	}
}

func quoteMySQLIdentifier(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

func quotePostgresIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
