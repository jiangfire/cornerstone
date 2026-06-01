package source

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mockPostgresSchemaQueries(mock sqlmock.Sqlmock, schemaName, tableName string) {
	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT column_name, data_type, udt_name, is_nullable, column_default, character_maximum_length
FROM information_schema.columns
WHERE table_schema = $1 AND table_name = $2
ORDER BY ordinal_position`)).
		WithArgs(schemaName, tableName).
		WillReturnRows(sqlmock.NewRows([]string{"column_name", "data_type", "udt_name", "is_nullable", "column_default", "character_maximum_length"}).
			AddRow("id", "integer", "int4", "NO", nil, nil).
			AddRow("name", "character varying", "varchar", "YES", nil, nil))

	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT kcu.column_name
FROM information_schema.table_constraints tc
JOIN information_schema.key_column_usage kcu
  ON tc.constraint_name = kcu.constraint_name
 AND tc.table_schema = kcu.table_schema
WHERE tc.table_schema = $1 AND tc.table_name = $2 AND tc.constraint_type = 'PRIMARY KEY'
ORDER BY kcu.ordinal_position`)).
		WithArgs(schemaName, tableName).
		WillReturnRows(sqlmock.NewRows([]string{"column_name"}).AddRow("id"))

	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT tc.constraint_name, kcu.column_name
FROM information_schema.table_constraints tc
JOIN information_schema.key_column_usage kcu
  ON tc.constraint_name = kcu.constraint_name
 AND tc.table_schema = kcu.table_schema
WHERE tc.table_schema = $1 AND tc.table_name = $2 AND tc.constraint_type = 'UNIQUE'
ORDER BY tc.constraint_name, kcu.ordinal_position`)).
		WithArgs(schemaName, tableName).
		WillReturnRows(sqlmock.NewRows([]string{"constraint_name", "column_name"}).AddRow("users_name_key", "name"))

	mock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(5)))
}

func TestPostgresSource_Connect_InvalidDSN(t *testing.T) {
	src := &PostgresSource{}
	err := src.Connect("invalid://dsn")
	require.Error(t, err)
}

func TestPostgresSource_Close_NilDB(t *testing.T) {
	src := &PostgresSource{db: nil}
	err := src.Close()
	assert.Nil(t, err)
}

func TestPostgresSource_ListDatabases(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	src := &PostgresSource{db: db}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY datname`)).
		WillReturnRows(sqlmock.NewRows([]string{"datname"}).AddRow("mydb").AddRow("postgres"))

	result, err := src.ListDatabases()
	require.NoError(t, err)
	assert.Equal(t, []string{"mydb", "postgres"}, result)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresSource_GetTableSchema(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	src := &PostgresSource{db: db, schemaName: "public"}
	mockPostgresSchemaQueries(mock, "public", "users")

	schema, err := src.GetTableSchema("", "users")
	require.NoError(t, err)
	assert.Equal(t, "users", schema.Name)
	require.Len(t, schema.Columns, 2)
	assert.Equal(t, "id", schema.Columns[0].Name)
	assert.Equal(t, "integer", schema.Columns[0].Type)
	assert.False(t, schema.Columns[0].Nullable)
	assert.True(t, schema.Columns[0].IsPrimaryKey)
	assert.Equal(t, "name", schema.Columns[1].Name)
	assert.Equal(t, "character varying", schema.Columns[1].Type)
	assert.True(t, schema.Columns[1].Nullable)
	assert.True(t, schema.Columns[1].IsUnique)
	assert.Equal(t, []string{"id"}, schema.PrimaryKey)
	require.Len(t, schema.UniqueKeys, 1)
	assert.Equal(t, []string{"name"}, schema.UniqueKeys[0])
	assert.Equal(t, int64(5), schema.RowEstimate)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresSource_QueryRows_Cursor(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	src := &PostgresSource{db: db, schemaName: "public"}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "public"."users" WHERE "id" > $1 ORDER BY "id" ASC LIMIT $2`)).
		WithArgs(int64(10), int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(int64(11), "alice"))

	rows, err := src.QueryRows("", "users", QueryOptions{
		Strategy:     StrategyCursor,
		CursorColumn: "id",
		CursorValue:  int64(10),
		Limit:        5,
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, int64(11), rows[0]["id"])
	assert.Equal(t, "alice", rows[0]["name"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresSource_QueryRows_WithFilter(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	src := &PostgresSource{db: db, schemaName: "public"}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "public"."users" WHERE (active = true) AND "id" > $1 ORDER BY "id" ASC LIMIT $2`)).
		WithArgs(int64(50), int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(int64(51), "bob"))

	rows, err := src.QueryRows("", "users", QueryOptions{
		Strategy:     StrategyCursor,
		CursorColumn: "id",
		CursorValue:  int64(50),
		Limit:        10,
		Filter:       "active = true",
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "bob", rows[0]["name"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresSource_RecommendPaginationStrategy_Cursor(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	src := &PostgresSource{db: db, schemaName: "public"}
	mockPostgresSchemaQueries(mock, "public", "users")

	strategy := src.RecommendPaginationStrategy("", "users")
	assert.Equal(t, StrategyCursor, strategy)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresSource_RecommendPaginationStrategy_Offset(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	src := &PostgresSource{db: db, schemaName: "public"}
	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT column_name, data_type, udt_name, is_nullable, column_default, character_maximum_length
FROM information_schema.columns
WHERE table_schema = $1 AND table_name = $2
ORDER BY ordinal_position`)).
		WithArgs("public", "users").
		WillReturnError(fmt.Errorf("table not found"))

	strategy := src.RecommendPaginationStrategy("", "users")
	assert.Equal(t, StrategyOffset, strategy)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresSource_EffectiveSchema_Default(t *testing.T) {
	src := &PostgresSource{schemaName: ""}
	assert.Equal(t, "public", src.effectiveSchema())
}

func TestPostgresSource_EffectiveSchema_Custom(t *testing.T) {
	src := &PostgresSource{schemaName: "custom"}
	assert.Equal(t, "custom", src.effectiveSchema())
}

func TestPostgresSource_EffectiveSchema_Whitespace(t *testing.T) {
	src := &PostgresSource{schemaName: "   "}
	assert.Equal(t, "public", src.effectiveSchema())
}

func TestBuildPostgresQuery_OffsetStrategy(t *testing.T) {
	query, args := buildPostgresQuery("public", "users", QueryOptions{
		Strategy: StrategyOffset,
		Limit:    10,
		Offset:   5,
	})
	assert.True(t, strings.Contains(query, "LIMIT $"))
	assert.True(t, strings.Contains(query, "OFFSET $"))
	assert.Equal(t, []interface{}{int64(10), int64(5)}, args)
}

func TestBuildPostgresQuery_CursorWithFilter(t *testing.T) {
	query, args := buildPostgresQuery("public", "users", QueryOptions{
		Strategy:     StrategyCursor,
		CursorColumn: "id",
		CursorValue:  int64(42),
		Limit:        100,
		Filter:       "active = true",
	})
	assert.True(t, strings.Contains(query, "WHERE"))
	assert.True(t, strings.Contains(query, "(active = true)"))
	assert.True(t, strings.Contains(query, `"id" > $`))
	assert.Equal(t, []interface{}{int64(42), int64(100)}, args)
}

func TestBuildPostgresQuery_NoFilterNoCursor(t *testing.T) {
	query, args := buildPostgresQuery("public", "items", QueryOptions{
		Strategy: StrategyOffset,
		Limit:    20,
		Offset:   0,
	})
	assert.False(t, strings.Contains(query, "WHERE"))
	assert.True(t, strings.Contains(query, "ORDER BY ctid ASC"))
	assert.Equal(t, []interface{}{int64(20), int64(0)}, args)
}

func TestPostgresRawType_Array(t *testing.T) {
	assert.Equal(t, "array", postgresRawType("ARRAY", ""))
}

func TestPostgresRawType_Normal(t *testing.T) {
	assert.Equal(t, "integer", postgresRawType("integer", ""))
}

func TestPostgresRawType_EmptyDataType(t *testing.T) {
	assert.Equal(t, "int4", postgresRawType("", "int4"))
}

func TestPostgresRawType_WhitespaceDataType(t *testing.T) {
	assert.Equal(t, "int4", postgresRawType("   ", "int4"))
}

func TestPostgresSource_ListDatabases_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	src := &PostgresSource{db: db}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY datname`)).
		WillReturnError(fmt.Errorf("connection lost"))

	_, err = src.ListDatabases()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list postgres databases")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresSource_EstimateRowCount_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	src := &PostgresSource{db: db, schemaName: "public"}
	mock.ExpectQuery("SELECT COUNT").
		WillReturnError(fmt.Errorf("table missing"))

	_, err = src.EstimateRowCount("", "missing_table")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "count postgres rows")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresSource_QueryRows_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	src := &PostgresSource{db: db, schemaName: "public"}
	mock.ExpectQuery("SELECT").
		WillReturnError(fmt.Errorf("query failed"))

	_, err = src.QueryRows("", "bogus", QueryOptions{Strategy: StrategyOffset, Limit: 10, Offset: 0})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "query postgres rows")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresSource_RecommendPaginationStrategy_NoSingleKey(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	src := &PostgresSource{db: db, schemaName: "public"}

	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT column_name, data_type, udt_name, is_nullable, column_default, character_maximum_length
FROM information_schema.columns
WHERE table_schema = $1 AND table_name = $2
ORDER BY ordinal_position`)).
		WithArgs("public", "users").
		WillReturnRows(sqlmock.NewRows([]string{"column_name", "data_type", "udt_name", "is_nullable", "column_default", "character_maximum_length"}).
			AddRow("a", "integer", "int4", "NO", nil, nil).
			AddRow("b", "integer", "int4", "NO", nil, nil))

	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT kcu.column_name
FROM information_schema.table_constraints tc
JOIN information_schema.key_column_usage kcu
  ON tc.constraint_name = kcu.constraint_name
 AND tc.table_schema = kcu.table_schema
WHERE tc.table_schema = $1 AND tc.table_name = $2 AND tc.constraint_type = 'PRIMARY KEY'
ORDER BY kcu.ordinal_position`)).
		WithArgs("public", "users").
		WillReturnRows(sqlmock.NewRows([]string{"column_name"}))

	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT tc.constraint_name, kcu.column_name
FROM information_schema.table_constraints tc
JOIN information_schema.key_column_usage kcu
  ON tc.constraint_name = kcu.constraint_name
 AND tc.table_schema = kcu.table_schema
WHERE tc.table_schema = $1 AND tc.table_name = $2 AND tc.constraint_type = 'UNIQUE'
ORDER BY tc.constraint_name, kcu.ordinal_position`)).
		WithArgs("public", "users").
		WillReturnRows(sqlmock.NewRows([]string{"constraint_name", "column_name"}).
			AddRow("uniq_ab", "a").
			AddRow("uniq_ab", "b"))

	mock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))

	strategy := src.RecommendPaginationStrategy("", "users")
	assert.Equal(t, StrategyOffset, strategy)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresSource_Close_WithDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	mock.ExpectClose()

	src := &PostgresSource{db: db}
	err = src.Close()
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresSource_ListTables_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	src := &PostgresSource{db: db, schemaName: "public"}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE' ORDER BY table_name`)).
		WithArgs("public").
		WillReturnError(fmt.Errorf("connection lost"))

	_, err = src.ListTables("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list postgres tables")
	require.NoError(t, mock.ExpectationsWereMet())
}
