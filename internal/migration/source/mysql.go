package source

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

type MySQLSource struct {
	db           *sql.DB
	databaseName string
}

func (s *MySQLSource) Connect(dsn string) error {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("connect mysql: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return fmt.Errorf("ping mysql: %w", err)
	}
	s.db = db
	s.databaseName = mysqlDatabaseNameFromDSN(dsn)
	return nil
}

func (s *MySQLSource) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *MySQLSource) ListDatabases() ([]string, error) {
	rows, err := s.db.Query(`SELECT schema_name FROM information_schema.schemata ORDER BY schema_name`)
	if err != nil {
		return nil, fmt.Errorf("list mysql databases: %w", err)
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		result = append(result, name)
	}
	return result, rows.Err()
}

func (s *MySQLSource) ListTables(dbName string) ([]string, error) {
	if strings.TrimSpace(dbName) == "" {
		dbName = s.databaseName
	}
	rows, err := s.db.Query(`SELECT table_name FROM information_schema.tables WHERE table_schema = ? AND table_type = 'BASE TABLE' ORDER BY table_name`, dbName)
	if err != nil {
		return nil, fmt.Errorf("list mysql tables: %w", err)
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		result = append(result, name)
	}
	return result, rows.Err()
}

func (s *MySQLSource) GetTableSchema(dbName, tableName string) (*TableSchema, error) {
	if strings.TrimSpace(dbName) == "" {
		dbName = s.databaseName
	}

	rows, err := s.db.Query(`
SELECT column_name, column_type, is_nullable, column_default, character_maximum_length, column_comment, column_key
FROM information_schema.columns
WHERE table_schema = ? AND table_name = ?
ORDER BY ordinal_position`, dbName, tableName)
	if err != nil {
		return nil, fmt.Errorf("read mysql columns: %w", err)
	}
	defer rows.Close()

	schema := &TableSchema{Name: tableName}
	for rows.Next() {
		var (
			name       string
			columnType string
			isNullable string
			defaultVal interface{}
			maxLength  sql.NullInt64
			comment    string
			columnKey  string
		)
		if err := rows.Scan(&name, &columnType, &isNullable, &defaultVal, &maxLength, &comment, &columnKey); err != nil {
			return nil, err
		}
		var length *int
		if maxLength.Valid {
			value := int(maxLength.Int64)
			length = &value
		}
		column := ColumnSchema{
			Name:         name,
			Type:         columnType,
			Nullable:     strings.EqualFold(isNullable, "YES"),
			DefaultValue: defaultVal,
			MaxLength:    length,
			Comment:      comment,
			IsPrimaryKey: strings.EqualFold(columnKey, "PRI"),
			IsUnique:     strings.EqualFold(columnKey, "UNI"),
		}
		if column.IsPrimaryKey {
			schema.PrimaryKey = append(schema.PrimaryKey, name)
		}
		schema.Columns = append(schema.Columns, column)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	schema.UniqueKeys, err = s.loadMySQLUniqueKeys(dbName, tableName)
	if err != nil {
		return nil, err
	}
	schema.RowEstimate, err = s.EstimateRowCount(dbName, tableName)
	if err != nil {
		return nil, err
	}
	return schema, nil
}

func (s *MySQLSource) loadMySQLUniqueKeys(dbName, tableName string) ([][]string, error) {
	rows, err := s.db.Query(`
SELECT index_name, column_name
FROM information_schema.statistics
WHERE table_schema = ? AND table_name = ? AND non_unique = 0
ORDER BY index_name, seq_in_index`, dbName, tableName)
	if err != nil {
		return nil, fmt.Errorf("read mysql unique keys: %w", err)
	}
	defer rows.Close()

	var (
		result        [][]string
		currentName   string
		currentFields []string
	)
	for rows.Next() {
		var indexName string
		var columnName string
		if err := rows.Scan(&indexName, &columnName); err != nil {
			return nil, err
		}
		if indexName != currentName {
			if len(currentFields) > 0 && currentName != "PRIMARY" {
				result = append(result, currentFields)
			}
			currentName = indexName
			currentFields = []string{}
		}
		currentFields = append(currentFields, columnName)
	}
	if len(currentFields) > 0 && currentName != "PRIMARY" {
		result = append(result, currentFields)
	}
	return result, rows.Err()
}

func (s *MySQLSource) EstimateRowCount(dbName, tableName string) (int64, error) {
	if strings.TrimSpace(dbName) == "" {
		dbName = s.databaseName
	}
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s", quoteMySQLIdentifier(dbName), quoteMySQLIdentifier(tableName))
	var count int64
	if err := s.db.QueryRow(query).Scan(&count); err != nil {
		return 0, fmt.Errorf("count mysql rows: %w", err)
	}
	return count, nil
}

func (s *MySQLSource) QueryRows(dbName, tableName string, opts QueryOptions) ([]map[string]interface{}, error) {
	if strings.TrimSpace(dbName) == "" {
		dbName = s.databaseName
	}
	query, args := buildMySQLQuery(dbName, tableName, opts)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query mysql rows: %w", err)
	}
	defer rows.Close()
	return scanRows(rows)
}

func (s *MySQLSource) RecommendPaginationStrategy(dbName, tableName string) PaginationStrategy {
	schema, err := s.GetTableSchema(dbName, tableName)
	if err != nil {
		return StrategyOffset
	}
	if len(schema.PrimaryKey) == 1 {
		return StrategyCursor
	}
	for _, key := range schema.UniqueKeys {
		if len(key) == 1 {
			return StrategyCursor
		}
	}
	return StrategyOffset
}

func buildMySQLQuery(dbName, tableName string, opts QueryOptions) (string, []interface{}) {
	query := fmt.Sprintf("SELECT * FROM %s.%s", quoteMySQLIdentifier(dbName), quoteMySQLIdentifier(tableName))
	args := make([]interface{}, 0, 4)
	var filters []string
	if strings.TrimSpace(opts.Filter) != "" {
		filters = append(filters, "("+opts.Filter+")")
	}
	if opts.Strategy == StrategyCursor && strings.TrimSpace(opts.CursorColumn) != "" && opts.CursorValue != nil {
		filters = append(filters, fmt.Sprintf("%s > ?", quoteMySQLIdentifier(opts.CursorColumn)))
		args = append(args, opts.CursorValue)
	}
	if len(filters) > 0 {
		query += " WHERE " + strings.Join(filters, " AND ")
	}
	if opts.Strategy == StrategyCursor && strings.TrimSpace(opts.CursorColumn) != "" {
		query += fmt.Sprintf(" ORDER BY %s ASC", quoteMySQLIdentifier(opts.CursorColumn))
	} else {
		query += " ORDER BY 1 ASC"
	}
	query += " LIMIT ?"
	args = append(args, opts.Limit)
	if opts.Strategy == StrategyOffset {
		query += " OFFSET ?"
		args = append(args, opts.Offset)
	}
	return query, args
}

func mysqlDatabaseNameFromDSN(dsn string) string {
	parts := strings.SplitN(dsn, "/", 2)
	if len(parts) != 2 {
		return ""
	}
	dbPart := parts[1]
	if idx := strings.Index(dbPart, "?"); idx >= 0 {
		dbPart = dbPart[:idx]
	}
	return dbPart
}
