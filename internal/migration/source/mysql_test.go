package source

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMySQLSource_ListTables(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	src := &MySQLSource{db: db, databaseName: "demo"}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT table_name FROM information_schema.tables WHERE table_schema = ? AND table_type = 'BASE TABLE' ORDER BY table_name`)).
		WithArgs("demo").
		WillReturnRows(sqlmock.NewRows([]string{"table_name"}).AddRow("orders").AddRow("users"))

	tables, err := src.ListTables("demo")
	require.NoError(t, err)
	assert.Equal(t, []string{"orders", "users"}, tables)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLSource_QueryRowsCursor(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	src := &MySQLSource{db: db, databaseName: "demo"}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `demo`.`users` WHERE `id` > ? ORDER BY `id` ASC LIMIT ?")).
		WithArgs(int64(10), int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(int64(11), "alice"))

	rows, err := src.QueryRows("demo", "users", QueryOptions{
		Strategy:     StrategyCursor,
		CursorColumn: "id",
		CursorValue:  int64(10),
		Limit:        2,
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, int64(11), rows[0]["id"])
	assert.Equal(t, "alice", rows[0]["name"])
	require.NoError(t, mock.ExpectationsWereMet())
}
