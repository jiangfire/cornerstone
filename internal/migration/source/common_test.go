package source

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeSQLValue_ByteSlice(t *testing.T) {
	result := normalizeSQLValue([]byte("hello"))
	assert.Equal(t, "hello", result)
}

func TestNormalizeSQLValue_Other(t *testing.T) {
	result := normalizeSQLValue(42)
	assert.Equal(t, 42, result)
}

func TestNormalizeSQLValue_String(t *testing.T) {
	result := normalizeSQLValue("world")
	assert.Equal(t, "world", result)
}

func TestNormalizeSQLValue_Nil(t *testing.T) {
	result := normalizeSQLValue(nil)
	assert.Nil(t, result)
}

func TestQuoteMySQLIdentifier(t *testing.T) {
	assert.Equal(t, "`users`", quoteMySQLIdentifier("users"))
	assert.Equal(t, "````", quoteMySQLIdentifier("`"))
	assert.Equal(t, "`table``with``backtick`", quoteMySQLIdentifier("table`with`backtick"))
}

func TestQuotePostgresIdentifier(t *testing.T) {
	assert.Equal(t, `"users"`, quotePostgresIdentifier("users"))
	assert.Equal(t, `""""`, quotePostgresIdentifier(`"`))
	assert.Equal(t, `"table""with""quote"`, quotePostgresIdentifier(`table"with"quote`))
}

func TestNewSource_Unsupported(t *testing.T) {
	_, err := NewSource("oracle")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported source type")
}

func TestNewSource_SQLite(t *testing.T) {
	src, err := NewSource("sqlite")
	require.NoError(t, err)
	_, ok := src.(*SQLiteSource)
	assert.True(t, ok)
}

func TestNewSource_MySQL(t *testing.T) {
	src, err := NewSource("mysql")
	require.NoError(t, err)
	_, ok := src.(*MySQLSource)
	assert.True(t, ok)
}

func TestNewSource_Postgres(t *testing.T) {
	src, err := NewSource("postgres")
	require.NoError(t, err)
	_, ok := src.(*PostgresSource)
	assert.True(t, ok)
}
