package source

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresSource_ListTables(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	src := &PostgresSource{db: db, schemaName: "public"}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE' ORDER BY table_name`)).
		WithArgs("public").
		WillReturnRows(sqlmock.NewRows([]string{"table_name"}).AddRow("audit_logs").AddRow("users"))

	tables, err := src.ListTables("")
	require.NoError(t, err)
	assert.Equal(t, []string{"audit_logs", "users"}, tables)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresSource_QueryRowsOffset(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	src := &PostgresSource{db: db, schemaName: "public"}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "public"."events" ORDER BY ctid ASC LIMIT $1 OFFSET $2`)).
		WithArgs(int64(5), int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(int64(1), "evt"))

	rows, err := src.QueryRows("", "events", QueryOptions{
		Strategy: StrategyOffset,
		Limit:    5,
		Offset:   10,
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "evt", rows[0]["name"])
	require.NoError(t, mock.ExpectationsWereMet())
}
