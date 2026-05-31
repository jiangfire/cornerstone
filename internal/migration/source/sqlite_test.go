package source

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/glebarez/go-sqlite"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSQLiteSource_ReadsSchemaAndRows(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "source.db")
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`
CREATE TABLE users (
	id INTEGER PRIMARY KEY,
	name TEXT NOT NULL,
	active INTEGER,
	created_at TEXT
);
INSERT INTO users (id, name, active, created_at) VALUES
	(1, 'alice', 1, '2026-05-31T10:00:00Z'),
	(2, 'bob', 0, '2026-05-31T11:00:00Z');

CREATE TABLE logs (
	message TEXT
);
INSERT INTO logs (message) VALUES ('hello'), ('world');
`)
	require.NoError(t, err)

	src, err := NewSource("sqlite")
	require.NoError(t, err)
	require.NoError(t, src.Connect(dbPath))
	t.Cleanup(func() { _ = src.Close() })

	tables, err := src.ListTables("")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"users", "logs"}, tables)

	schema, err := src.GetTableSchema("", "users")
	require.NoError(t, err)
	assert.Equal(t, []string{"id"}, schema.PrimaryKey)
	assert.Len(t, schema.Columns, 4)
	assert.Equal(t, int64(2), schema.RowEstimate)

	assert.Equal(t, StrategyCursor, src.RecommendPaginationStrategy("", "users"))
	assert.Equal(t, StrategyOffset, src.RecommendPaginationStrategy("", "logs"))

	rows, err := src.QueryRows("", "users", QueryOptions{
		Strategy:     StrategyCursor,
		CursorColumn: "id",
		Limit:        1,
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, int64(1), rows[0]["id"])
	assert.Equal(t, "alice", rows[0]["name"])

	rows, err = src.QueryRows("", "users", QueryOptions{
		Strategy:     StrategyCursor,
		CursorColumn: "id",
		CursorValue:  int64(1),
		Limit:        10,
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, int64(2), rows[0]["id"])

	rows, err = src.QueryRows("", "logs", QueryOptions{
		Strategy: StrategyOffset,
		Offset:   1,
		Limit:    10,
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "world", rows[0]["message"])
}
