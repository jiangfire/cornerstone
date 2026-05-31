package source

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	// Register SQLite driver for database/sql source connections.
	_ "github.com/glebarez/go-sqlite"
)

type SQLiteSource struct {
	db           *sql.DB
	databaseName string
}

func (s *SQLiteSource) Connect(dsn string) error {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("connect sqlite: %w", err)
	}
	if _, err := db.Exec("PRAGMA query_only = ON"); err != nil {
		_ = db.Close()
		return fmt.Errorf("enable sqlite read only hint: %w", err)
	}
	s.db = db
	s.databaseName = deriveSQLiteDatabaseName(dsn)
	return nil
}

func (s *SQLiteSource) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteSource) ListDatabases() ([]string, error) {
	return []string{s.databaseName}, nil
}

func (s *SQLiteSource) ListTables(_ string) ([]string, error) {
	rows, err := s.db.Query(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list sqlite tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan sqlite table: %w", err)
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

func (s *SQLiteSource) GetTableSchema(_ string, tableName string) (*TableSchema, error) {
	rows, err := s.db.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, quoteSQLiteIdentifier(tableName)))
	if err != nil {
		return nil, fmt.Errorf("read sqlite schema: %w", err)
	}
	defer rows.Close()

	schema := &TableSchema{Name: tableName}
	for rows.Next() {
		var (
			cid        int
			name       string
			rawType    string
			notNull    int
			defaultVal interface{}
			pk         int
		)
		if err := rows.Scan(&cid, &name, &rawType, &notNull, &defaultVal, &pk); err != nil {
			return nil, fmt.Errorf("scan sqlite column schema: %w", err)
		}
		column := ColumnSchema{
			Name:         name,
			Type:         rawType,
			Nullable:     notNull == 0,
			DefaultValue: defaultVal,
			IsPrimaryKey: pk > 0,
		}
		if column.IsPrimaryKey {
			schema.PrimaryKey = append(schema.PrimaryKey, name)
		}
		schema.Columns = append(schema.Columns, column)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	uniqueKeys, uniqueCols, err := s.sqliteUniqueKeys(tableName)
	if err != nil {
		return nil, err
	}
	schema.UniqueKeys = uniqueKeys
	for idx := range schema.Columns {
		if uniqueCols[schema.Columns[idx].Name] {
			schema.Columns[idx].IsUnique = true
		}
	}
	schema.RowEstimate, err = s.EstimateRowCount("", tableName)
	if err != nil {
		return nil, err
	}
	return schema, nil
}

func (s *SQLiteSource) sqliteUniqueKeys(tableName string) ([][]string, map[string]bool, error) {
	rows, err := s.db.Query(fmt.Sprintf(`PRAGMA index_list(%s)`, quoteSQLiteIdentifier(tableName)))
	if err != nil {
		return nil, nil, fmt.Errorf("read sqlite indexes: %w", err)
	}
	defer rows.Close()

	var uniqueKeys [][]string
	uniqueCols := map[string]bool{}
	for rows.Next() {
		var (
			seq     int
			name    string
			unique  int
			origin  string
			partial int
		)
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			return nil, nil, fmt.Errorf("scan sqlite index: %w", err)
		}
		if unique == 0 || origin == "pk" {
			continue
		}
		indexRows, err := s.db.Query(fmt.Sprintf(`PRAGMA index_info(%s)`, quoteSQLiteIdentifier(name)))
		if err != nil {
			return nil, nil, fmt.Errorf("read sqlite index info: %w", err)
		}
		var columns []string
		for indexRows.Next() {
			var (
				seqNo int
				cid   int
				col   string
			)
			if err := indexRows.Scan(&seqNo, &cid, &col); err != nil {
				_ = indexRows.Close()
				return nil, nil, fmt.Errorf("scan sqlite index info: %w", err)
			}
			columns = append(columns, col)
			uniqueCols[col] = true
		}
		_ = indexRows.Close()
		if len(columns) > 0 {
			uniqueKeys = append(uniqueKeys, columns)
		}
	}
	return uniqueKeys, uniqueCols, rows.Err()
}

func (s *SQLiteSource) EstimateRowCount(_ string, tableName string) (int64, error) {
	// #nosec G201 -- identifiers are quoted via quoteSQLiteIdentifier before interpolation.
	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s`, quoteSQLiteIdentifier(tableName))
	var count int64
	if err := s.db.QueryRow(query).Scan(&count); err != nil {
		return 0, fmt.Errorf("count sqlite rows: %w", err)
	}
	return count, nil
}

func (s *SQLiteSource) QueryRows(_ string, tableName string, opts QueryOptions) ([]map[string]interface{}, error) {
	query, args := buildSQLiteQuery(tableName, opts)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query sqlite rows: %w", err)
	}
	defer rows.Close()
	return scanRows(rows)
}

func (s *SQLiteSource) RecommendPaginationStrategy(_ string, tableName string) PaginationStrategy {
	schema, err := s.GetTableSchema("", tableName)
	if err != nil {
		return StrategyOffset
	}
	if len(schema.PrimaryKey) == 1 {
		return StrategyCursor
	}
	for _, uniqueKey := range schema.UniqueKeys {
		if len(uniqueKey) == 1 {
			return StrategyCursor
		}
	}
	return StrategyOffset
}

func buildSQLiteQuery(tableName string, opts QueryOptions) (string, []interface{}) {
	base := fmt.Sprintf(`SELECT * FROM %s`, quoteSQLiteIdentifier(tableName))
	args := make([]interface{}, 0, 3)
	var filters []string
	if strings.TrimSpace(opts.Filter) != "" {
		filters = append(filters, "("+opts.Filter+")")
	}
	switch opts.Strategy {
	case StrategyCursor:
		if strings.TrimSpace(opts.CursorColumn) != "" && opts.CursorValue != nil {
			filters = append(filters, fmt.Sprintf(`%s > ?`, quoteSQLiteIdentifier(opts.CursorColumn)))
			args = append(args, opts.CursorValue)
		}
	case StrategyOffset:
	default:
	}
	if len(filters) > 0 {
		base += " WHERE " + strings.Join(filters, " AND ")
	}
	if opts.Strategy == StrategyCursor && strings.TrimSpace(opts.CursorColumn) != "" {
		base += fmt.Sprintf(` ORDER BY %s ASC`, quoteSQLiteIdentifier(opts.CursorColumn))
	} else {
		base += ` ORDER BY rowid ASC`
	}
	base += ` LIMIT ?`
	args = append(args, opts.Limit)
	if opts.Strategy == StrategyOffset {
		base += ` OFFSET ?`
		args = append(args, opts.Offset)
	}
	return base, args
}

func quoteSQLiteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func deriveSQLiteDatabaseName(dsn string) string {
	base := filepath.Base(dsn)
	if base == "." || base == "" || dsn == ":memory:" {
		return "main"
	}
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}
