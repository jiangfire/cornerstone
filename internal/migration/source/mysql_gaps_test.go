package source

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mockMySQLColumnsQuery(mock sqlmock.Sqlmock, dbName, tableName string) {
	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT column_name, column_type, is_nullable, column_default, character_maximum_length, column_comment, column_key
FROM information_schema.columns
WHERE table_schema = ? AND table_name = ?
ORDER BY ordinal_position`)).
		WithArgs(dbName, tableName).
		WillReturnRows(sqlmock.NewRows([]string{"column_name", "column_type", "is_nullable", "column_default", "character_maximum_length", "column_comment", "column_key"}).
			AddRow("id", "int", "NO", nil, nil, "", "PRI").
			AddRow("name", "varchar(255)", "YES", nil, sql.NullInt64{Int64: 255, Valid: true}, "user name", "UNI"))

	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT index_name, column_name
FROM information_schema.statistics
WHERE table_schema = ? AND table_name = ? AND non_unique = 0
ORDER BY index_name, seq_in_index`)).
		WithArgs(dbName, tableName).
		WillReturnRows(sqlmock.NewRows([]string{"index_name", "column_name"}).
			AddRow("PRIMARY", "id").
			AddRow("uniq_name", "name"))

	mock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(10)))
}

func TestMySQLSource_Connect_InvalidDSN(t *testing.T) {
	src := &MySQLSource{}
	err := src.Connect("invalid://dsn")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connect mysql")
}

func TestMySQLSource_Close_NilDB(t *testing.T) {
	src := &MySQLSource{db: nil}
	err := src.Close()
	assert.Nil(t, err)
}

func TestMySQLSource_ListDatabases(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	src := &MySQLSource{db: db}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT schema_name FROM information_schema.schemata ORDER BY schema_name`)).
		WillReturnRows(sqlmock.NewRows([]string{"schema_name"}).AddRow("db1").AddRow("db2"))

	result, err := src.ListDatabases()
	require.NoError(t, err)
	assert.Equal(t, []string{"db1", "db2"}, result)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLSource_ListTables_EmptyDBName(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	src := &MySQLSource{db: db, databaseName: "fallbackdb"}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT table_name FROM information_schema.tables WHERE table_schema = ? AND table_type = 'BASE TABLE' ORDER BY table_name`)).
		WithArgs("fallbackdb").
		WillReturnRows(sqlmock.NewRows([]string{"table_name"}).AddRow("orders"))

	tables, err := src.ListTables("")
	require.NoError(t, err)
	assert.Equal(t, []string{"orders"}, tables)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLSource_ListTables_WhitespaceDBName(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	src := &MySQLSource{db: db, databaseName: "fallbackdb"}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT table_name FROM information_schema.tables WHERE table_schema = ? AND table_type = 'BASE TABLE' ORDER BY table_name`)).
		WithArgs("fallbackdb").
		WillReturnRows(sqlmock.NewRows([]string{"table_name"}).AddRow("users"))

	tables, err := src.ListTables("   ")
	require.NoError(t, err)
	assert.Equal(t, []string{"users"}, tables)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLSource_GetTableSchema(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	src := &MySQLSource{db: db, databaseName: "testdb"}
	mockMySQLColumnsQuery(mock, "testdb", "users")

	schema, err := src.GetTableSchema("testdb", "users")
	require.NoError(t, err)
	assert.Equal(t, "users", schema.Name)
	require.Len(t, schema.Columns, 2)
	assert.Equal(t, "id", schema.Columns[0].Name)
	assert.Equal(t, "int", schema.Columns[0].Type)
	assert.False(t, schema.Columns[0].Nullable)
	assert.True(t, schema.Columns[0].IsPrimaryKey)
	assert.Equal(t, "name", schema.Columns[1].Name)
	assert.Equal(t, "varchar(255)", schema.Columns[1].Type)
	assert.True(t, schema.Columns[1].Nullable)
	assert.True(t, schema.Columns[1].IsUnique)
	require.NotNil(t, schema.Columns[1].MaxLength)
	assert.Equal(t, 255, *schema.Columns[1].MaxLength)
	assert.Equal(t, []string{"id"}, schema.PrimaryKey)
	require.Len(t, schema.UniqueKeys, 1)
	assert.Equal(t, []string{"name"}, schema.UniqueKeys[0])
	assert.Equal(t, int64(10), schema.RowEstimate)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLSource_GetTableSchema_EmptyDBName(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	src := &MySQLSource{db: db, databaseName: "fallbackdb"}
	mockMySQLColumnsQuery(mock, "fallbackdb", "users")

	schema, err := src.GetTableSchema("", "users")
	require.NoError(t, err)
	assert.Equal(t, "users", schema.Name)
	require.Len(t, schema.Columns, 2)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLSource_EstimateRowCount_EmptyDBName(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	src := &MySQLSource{db: db, databaseName: "mydb"}
	mock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(42)))

	count, err := src.EstimateRowCount("", "orders")
	require.NoError(t, err)
	assert.Equal(t, int64(42), count)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLSource_QueryRows_Offset(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	src := &MySQLSource{db: db, databaseName: "testdb"}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `testdb`.`items` WHERE (status = 'active') ORDER BY 1 ASC LIMIT ? OFFSET ?")).
		WithArgs(int64(10), int64(20)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(int64(1), "item1").AddRow(int64(2), "item2"))

	rows, err := src.QueryRows("testdb", "items", QueryOptions{
		Strategy: StrategyOffset,
		Limit:    10,
		Offset:   20,
		Filter:   "status = 'active'",
	})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, int64(1), rows[0]["id"])
	assert.Equal(t, "item2", rows[1]["name"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLSource_QueryRows_WithFilter(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	src := &MySQLSource{db: db, databaseName: "testdb"}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `testdb`.`users` WHERE (age > 18) AND `id` > ? ORDER BY `id` ASC LIMIT ?")).
		WithArgs(int64(100), int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(int64(101), "bob"))

	rows, err := src.QueryRows("testdb", "users", QueryOptions{
		Strategy:     StrategyCursor,
		CursorColumn: "id",
		CursorValue:  int64(100),
		Limit:        5,
		Filter:       "age > 18",
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "bob", rows[0]["name"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLSource_QueryRows_EmptyDBName(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	src := &MySQLSource{db: db, databaseName: "fallbackdb"}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `fallbackdb`.`users` ORDER BY 1 ASC LIMIT ? OFFSET ?")).
		WithArgs(int64(10), int64(0)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	rows, err := src.QueryRows("", "users", QueryOptions{
		Strategy: StrategyOffset,
		Limit:    10,
		Offset:   0,
	})
	require.NoError(t, err)
	assert.Empty(t, rows)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLSource_RecommendPaginationStrategy_Cursor(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	src := &MySQLSource{db: db, databaseName: "testdb"}
	mockMySQLColumnsQuery(mock, "testdb", "users")

	strategy := src.RecommendPaginationStrategy("testdb", "users")
	assert.Equal(t, StrategyCursor, strategy)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLSource_RecommendPaginationStrategy_Offset(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	src := &MySQLSource{db: db, databaseName: "testdb"}
	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT column_name, column_type, is_nullable, column_default, character_maximum_length, column_comment, column_key
FROM information_schema.columns
WHERE table_schema = ? AND table_name = ?
ORDER BY ordinal_position`)).
		WithArgs("testdb", "users").
		WillReturnError(fmt.Errorf("table not found"))

	strategy := src.RecommendPaginationStrategy("testdb", "users")
	assert.Equal(t, StrategyOffset, strategy)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLSource_RecommendPaginationStrategy_NoSingleKey(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	src := &MySQLSource{db: db, databaseName: "testdb"}

	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT column_name, column_type, is_nullable, column_default, character_maximum_length, column_comment, column_key
FROM information_schema.columns
WHERE table_schema = ? AND table_name = ?
ORDER BY ordinal_position`)).
		WithArgs("testdb", "users").
		WillReturnRows(sqlmock.NewRows([]string{"column_name", "column_type", "is_nullable", "column_default", "character_maximum_length", "column_comment", "column_key"}).
			AddRow("a", "int", "NO", nil, nil, "", "").
			AddRow("b", "int", "NO", nil, nil, "", ""))

	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT index_name, column_name
FROM information_schema.statistics
WHERE table_schema = ? AND table_name = ? AND non_unique = 0
ORDER BY index_name, seq_in_index`)).
		WithArgs("testdb", "users").
		WillReturnRows(sqlmock.NewRows([]string{"index_name", "column_name"}).
			AddRow("uniq_ab", "a").
			AddRow("uniq_ab", "b"))

	mock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))

	strategy := src.RecommendPaginationStrategy("testdb", "users")
	assert.Equal(t, StrategyOffset, strategy)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLDatabaseNameFromDSN(t *testing.T) {
	tests := []struct {
		dsn    string
		expect string
	}{
		{"user:pass@tcp(host:3306)/mydb", "mydb"},
		{"user:pass@tcp(host:3306)/mydb?charset=utf8", "mydb"},
		{"user:pass@tcp(host:3306)/", ""},
		{"", ""},
		{"noslash", ""},
		{"/justdb", "justdb"},
		{"/justdb?param=val", "justdb"},
	}
	for _, tt := range tests {
		t.Run(tt.dsn, func(t *testing.T) {
			assert.Equal(t, tt.expect, mysqlDatabaseNameFromDSN(tt.dsn))
		})
	}
}

func TestBuildMySQLQuery_OffsetStrategy(t *testing.T) {
	query, args := buildMySQLQuery("mydb", "users", QueryOptions{
		Strategy: StrategyOffset,
		Limit:    10,
		Offset:   5,
	})
	assert.True(t, strings.Contains(query, "LIMIT ?"))
	assert.True(t, strings.Contains(query, "OFFSET ?"))
	assert.Equal(t, []interface{}{int64(10), int64(5)}, args)
}

func TestBuildMySQLQuery_CursorWithFilter(t *testing.T) {
	query, args := buildMySQLQuery("mydb", "users", QueryOptions{
		Strategy:     StrategyCursor,
		CursorColumn: "id",
		CursorValue:  int64(42),
		Limit:        100,
		Filter:       "active = 1",
	})
	assert.True(t, strings.Contains(query, "WHERE"))
	assert.True(t, strings.Contains(query, "(active = 1)"))
	assert.True(t, strings.Contains(query, "`id` > ?"))
	assert.Equal(t, []interface{}{int64(42), int64(100)}, args)
}

func TestBuildMySQLQuery_NoFilterNoCursor(t *testing.T) {
	query, args := buildMySQLQuery("mydb", "items", QueryOptions{
		Strategy: StrategyOffset,
		Limit:    20,
		Offset:   0,
	})
	assert.False(t, strings.Contains(query, "WHERE"))
	assert.True(t, strings.Contains(query, "ORDER BY 1 ASC"))
	assert.Equal(t, []interface{}{int64(20), int64(0)}, args)
}

func TestMySQLSource_ListDatabases_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	src := &MySQLSource{db: db}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT schema_name FROM information_schema.schemata ORDER BY schema_name`)).
		WillReturnError(fmt.Errorf("connection lost"))

	_, err = src.ListDatabases()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list mysql databases")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLSource_EstimateRowCount_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	src := &MySQLSource{db: db, databaseName: "testdb"}
	mock.ExpectQuery("SELECT COUNT").
		WillReturnError(fmt.Errorf("table missing"))

	_, err = src.EstimateRowCount("testdb", "missing_table")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "count mysql rows")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLSource_QueryRows_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	src := &MySQLSource{db: db, databaseName: "testdb"}
	mock.ExpectQuery("SELECT").
		WillReturnError(fmt.Errorf("query failed"))

	_, err = src.QueryRows("testdb", "bogus", QueryOptions{Strategy: StrategyOffset, Limit: 10, Offset: 0})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "query mysql rows")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLSource_GetTableSchema_NoUniqueKeys(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	src := &MySQLSource{db: db, databaseName: "testdb"}

	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT column_name, column_type, is_nullable, column_default, character_maximum_length, column_comment, column_key
FROM information_schema.columns
WHERE table_schema = ? AND table_name = ?
ORDER BY ordinal_position`)).
		WithArgs("testdb", "logs").
		WillReturnRows(sqlmock.NewRows([]string{"column_name", "column_type", "is_nullable", "column_default", "character_maximum_length", "column_comment", "column_key"}).
			AddRow("id", "int", "NO", nil, nil, "", "PRI"))

	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT index_name, column_name
FROM information_schema.statistics
WHERE table_schema = ? AND table_name = ? AND non_unique = 0
ORDER BY index_name, seq_in_index`)).
		WithArgs("testdb", "logs").
		WillReturnRows(sqlmock.NewRows([]string{"index_name", "column_name"}).
			AddRow("PRIMARY", "id"))

	mock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(100)))

	schema, err := src.GetTableSchema("testdb", "logs")
	require.NoError(t, err)
	assert.Equal(t, []string{"id"}, schema.PrimaryKey)
	assert.Empty(t, schema.UniqueKeys)
	require.NoError(t, mock.ExpectationsWereMet())
}
