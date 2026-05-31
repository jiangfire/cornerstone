package source

import (
	"database/sql"
	"fmt"
	"strings"

	// Register pgx stdlib driver for database/sql source connections.
	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresSource struct {
	db         *sql.DB
	schemaName string
}

func (s *PostgresSource) Connect(dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return fmt.Errorf("ping postgres: %w", err)
	}
	s.db = db
	s.schemaName = "public"
	return nil
}

func (s *PostgresSource) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *PostgresSource) ListDatabases() ([]string, error) {
	rows, err := s.db.Query(`SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY datname`)
	if err != nil {
		return nil, fmt.Errorf("list postgres databases: %w", err)
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

func (s *PostgresSource) ListTables(_ string) ([]string, error) {
	schema := s.effectiveSchema()
	rows, err := s.db.Query(`SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE' ORDER BY table_name`, schema)
	if err != nil {
		return nil, fmt.Errorf("list postgres tables: %w", err)
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

func (s *PostgresSource) GetTableSchema(_ string, tableName string) (*TableSchema, error) {
	schemaName := s.effectiveSchema()
	rows, err := s.db.Query(`
SELECT column_name, data_type, udt_name, is_nullable, column_default, character_maximum_length
FROM information_schema.columns
WHERE table_schema = $1 AND table_name = $2
ORDER BY ordinal_position`, schemaName, tableName)
	if err != nil {
		return nil, fmt.Errorf("read postgres columns: %w", err)
	}
	defer rows.Close()

	pkColumns, err := s.loadPostgresPrimaryKey(schemaName, tableName)
	if err != nil {
		return nil, err
	}
	uniqueKeys, err := s.loadPostgresUniqueKeys(schemaName, tableName)
	if err != nil {
		return nil, err
	}
	pkSet := make(map[string]bool, len(pkColumns))
	for _, column := range pkColumns {
		pkSet[column] = true
	}
	uniqueSet := map[string]bool{}
	for _, key := range uniqueKeys {
		if len(key) == 1 {
			uniqueSet[key[0]] = true
		}
	}

	schema := &TableSchema{Name: tableName, PrimaryKey: pkColumns, UniqueKeys: uniqueKeys}
	for rows.Next() {
		var (
			name       string
			dataType   string
			udtName    string
			isNullable string
			defaultVal interface{}
			maxLength  sql.NullInt64
		)
		if err := rows.Scan(&name, &dataType, &udtName, &isNullable, &defaultVal, &maxLength); err != nil {
			return nil, err
		}
		var length *int
		if maxLength.Valid {
			value := int(maxLength.Int64)
			length = &value
		}
		rawType := postgresRawType(dataType, udtName)
		schema.Columns = append(schema.Columns, ColumnSchema{
			Name:         name,
			Type:         rawType,
			Nullable:     strings.EqualFold(isNullable, "YES"),
			DefaultValue: defaultVal,
			MaxLength:    length,
			IsPrimaryKey: pkSet[name],
			IsUnique:     uniqueSet[name],
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	schema.RowEstimate, err = s.EstimateRowCount("", tableName)
	if err != nil {
		return nil, err
	}
	return schema, nil
}

func (s *PostgresSource) loadPostgresPrimaryKey(schemaName, tableName string) ([]string, error) {
	rows, err := s.db.Query(`
SELECT kcu.column_name
FROM information_schema.table_constraints tc
JOIN information_schema.key_column_usage kcu
  ON tc.constraint_name = kcu.constraint_name
 AND tc.table_schema = kcu.table_schema
WHERE tc.table_schema = $1 AND tc.table_name = $2 AND tc.constraint_type = 'PRIMARY KEY'
ORDER BY kcu.ordinal_position`, schemaName, tableName)
	if err != nil {
		return nil, fmt.Errorf("read postgres primary key: %w", err)
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

func (s *PostgresSource) loadPostgresUniqueKeys(schemaName, tableName string) ([][]string, error) {
	rows, err := s.db.Query(`
SELECT tc.constraint_name, kcu.column_name
FROM information_schema.table_constraints tc
JOIN information_schema.key_column_usage kcu
  ON tc.constraint_name = kcu.constraint_name
 AND tc.table_schema = kcu.table_schema
WHERE tc.table_schema = $1 AND tc.table_name = $2 AND tc.constraint_type = 'UNIQUE'
ORDER BY tc.constraint_name, kcu.ordinal_position`, schemaName, tableName)
	if err != nil {
		return nil, fmt.Errorf("read postgres unique keys: %w", err)
	}
	defer rows.Close()

	var (
		result        [][]string
		currentName   string
		currentFields []string
	)
	for rows.Next() {
		var constraintName string
		var columnName string
		if err := rows.Scan(&constraintName, &columnName); err != nil {
			return nil, err
		}
		if constraintName != currentName {
			if len(currentFields) > 0 {
				result = append(result, currentFields)
			}
			currentName = constraintName
			currentFields = []string{}
		}
		currentFields = append(currentFields, columnName)
	}
	if len(currentFields) > 0 {
		result = append(result, currentFields)
	}
	return result, rows.Err()
}

func (s *PostgresSource) EstimateRowCount(_ string, tableName string) (int64, error) {
	// #nosec G201 -- identifiers are quoted via quotePostgresIdentifier before interpolation.
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s", quotePostgresIdentifier(s.effectiveSchema()), quotePostgresIdentifier(tableName))
	var count int64
	if err := s.db.QueryRow(query).Scan(&count); err != nil {
		return 0, fmt.Errorf("count postgres rows: %w", err)
	}
	return count, nil
}

func (s *PostgresSource) QueryRows(_ string, tableName string, opts QueryOptions) ([]map[string]interface{}, error) {
	query, args := buildPostgresQuery(s.effectiveSchema(), tableName, opts)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query postgres rows: %w", err)
	}
	defer rows.Close()
	return scanRows(rows)
}

func (s *PostgresSource) RecommendPaginationStrategy(_ string, tableName string) PaginationStrategy {
	schema, err := s.GetTableSchema("", tableName)
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

func (s *PostgresSource) effectiveSchema() string {
	if strings.TrimSpace(s.schemaName) != "" {
		return s.schemaName
	}
	return "public"
}

func buildPostgresQuery(schemaName, tableName string, opts QueryOptions) (string, []interface{}) {
	query := fmt.Sprintf("SELECT * FROM %s.%s", quotePostgresIdentifier(schemaName), quotePostgresIdentifier(tableName))
	args := make([]interface{}, 0, 4)
	addArg := func(value interface{}) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	var filters []string
	if strings.TrimSpace(opts.Filter) != "" {
		filters = append(filters, "("+opts.Filter+")")
	}
	if opts.Strategy == StrategyCursor && strings.TrimSpace(opts.CursorColumn) != "" && opts.CursorValue != nil {
		filters = append(filters, fmt.Sprintf("%s > %s", quotePostgresIdentifier(opts.CursorColumn), addArg(opts.CursorValue)))
	}
	if len(filters) > 0 {
		query += " WHERE " + strings.Join(filters, " AND ")
	}
	if opts.Strategy == StrategyCursor && strings.TrimSpace(opts.CursorColumn) != "" {
		query += fmt.Sprintf(" ORDER BY %s ASC", quotePostgresIdentifier(opts.CursorColumn))
	} else {
		query += " ORDER BY ctid ASC"
	}
	query += " LIMIT " + addArg(opts.Limit)
	if opts.Strategy == StrategyOffset {
		query += " OFFSET " + addArg(opts.Offset)
	}
	return query, args
}

func postgresRawType(dataType, udtName string) string {
	if strings.EqualFold(dataType, "ARRAY") {
		return "array"
	}
	if strings.TrimSpace(dataType) != "" {
		return dataType
	}
	return udtName
}
