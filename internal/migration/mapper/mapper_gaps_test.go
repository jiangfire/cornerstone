package mapper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTypeMapper_Dispatches(t *testing.T) {
	cases := []struct {
		name       string
		dbType     string
		overrides  map[string]string
		rawType    string
		wantType   string
		wantWarn   bool
	}{
		{"mysql", "mysql", nil, "int(11)", "number", false},
		{"postgres", "postgres", nil, "boolean", "boolean", false},
		{"sqlite", "sqlite", nil, "INTEGER", "number", false},
		{"fallback unknown db", "cockroach", nil, "uuid", "string", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := NewTypeMapper(tc.dbType, tc.overrides)
			gotType, gotWarn := m.Map(tc.rawType)
			assert.Equal(t, tc.wantType, gotType)
			if tc.wantWarn {
				assert.NotEmpty(t, gotWarn)
			} else {
				assert.Empty(t, gotWarn)
			}
		})
	}
}

func TestNewTypeMapper_OverridesApplied(t *testing.T) {
	m := NewTypeMapper("mysql", map[string]string{
		"custom_type": "number",
	})
	gotType, gotWarn := m.Map("custom_type")
	assert.Equal(t, "number", gotType)
	assert.Empty(t, gotWarn)
}

func TestRawTypeOrType(t *testing.T) {
	assert.Equal(t, "unknown", rawTypeOrType(""))
	assert.Equal(t, "unknown", rawTypeOrType("   "))
	assert.Equal(t, "mysql", rawTypeOrType("mysql"))
	assert.Equal(t, "postgres", rawTypeOrType("postgres"))
}

func TestMySQLMapper_TableDriven(t *testing.T) {
	m := NewTypeMapper("mysql", nil)
	require.NotNil(t, m)

	cases := []struct {
		name     string
		rawType  string
		wantType string
		wantWarn bool
	}{
		{"tinyint1 bool", "tinyint(1)", "boolean", false},
		{"varchar", "varchar(255)", "string", false},
		{"char", "char(10)", "string", false},
		{"tinytext", "tinytext", "string", false},
		{"text", "text", "text", false},
		{"mediumtext", "mediumtext", "text", false},
		{"longtext", "longtext", "text", false},
		{"int", "int(11)", "number", false},
		{"bigint", "bigint(20)", "number", false},
		{"smallint", "smallint(6)", "number", false},
		{"float", "float", "number", false},
		{"double", "double", "number", false},
		{"decimal", "decimal(10,2)", "number", false},
		{"numeric", "numeric(10,2)", "number", false},
		{"date", "date", "date", false},
		{"datetime", "datetime", "date", false},
		{"timestamp", "timestamp", "datetime", false},
		{"json", "json", "json", false},
		{"enum", "enum('a','b')", "list", false},
		{"set", "set('x','y')", "list", false},
		{"blob fallback", "blob", "string", true},
		{"binary fallback", "binary(16)", "string", true},
		{"varbinary fallback", "varbinary(255)", "string", true},
		{"unknown fallback", "geometry", "string", true},
		{"int unsigned", "int unsigned", "number", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotType, gotWarn := m.Map(tc.rawType)
			assert.Equal(t, tc.wantType, gotType)
			if tc.wantWarn {
				assert.NotEmpty(t, gotWarn)
			} else {
				assert.Empty(t, gotWarn)
			}
		})
	}
}

func TestPostgresMapper_TableDriven(t *testing.T) {
	m := NewTypeMapper("postgres", nil)
	require.NotNil(t, m)

	cases := []struct {
		name     string
		rawType  string
		wantType string
		wantWarn bool
	}{
		{"character varying", "character varying(255)", "string", false},
		{"varchar", "varchar(100)", "string", false},
		{"character", "character(10)", "string", false},
		{"char", "char(5)", "string", false},
		{"text", "text", "text", false},
		{"integer", "integer", "number", false},
		{"bigint", "bigint", "number", false},
		{"smallint", "smallint", "number", false},
		{"real", "real", "number", false},
		{"double precision", "double precision", "number", false},
		{"numeric", "numeric(10,2)", "number", false},
		{"decimal", "decimal(8,4)", "number", false},
		{"boolean", "boolean", "boolean", false},
		{"date", "date", "date", false},
		{"timestamp", "timestamp without time zone", "datetime", false},
		{"timestamptz", "timestamptz", "datetime", false},
		{"timestamp with tz", "timestamp with time zone", "datetime", false},
		{"timestamp without tz", "timestamp without time zone", "datetime", false},
		{"json", "json", "json", false},
		{"jsonb", "jsonb", "json", false},
		{"array", "array", "list", false},
		{"bytea fallback", "bytea", "string", true},
		{"uuid", "uuid", "string", false},
		{"inet", "inet", "string", false},
		{"cidr", "cidr", "string", false},
		{"macaddr", "macaddr", "string", false},
		{"unknown fallback", "point", "string", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotType, gotWarn := m.Map(tc.rawType)
			assert.Equal(t, tc.wantType, gotType)
			if tc.wantWarn {
				assert.NotEmpty(t, gotWarn)
			} else {
				assert.Empty(t, gotWarn)
			}
		})
	}
}

func TestSQLiteMapper_TableDriven(t *testing.T) {
	m := NewTypeMapper("sqlite", nil)
	require.NotNil(t, m)

	cases := []struct {
		name     string
		rawType  string
		wantType string
		wantWarn bool
	}{
		{"tinyint1 bool", "tinyint(1)", "boolean", false},
		{"boolean keyword", "BOOLEAN", "boolean", false},
		{"text", "TEXT", "string", false},
		{"char", "CHARACTER", "string", false},
		{"clob", "CLOB", "string", false},
		{"json", "JSON", "json", false},
		{"integer", "INTEGER", "number", false},
		{"int", "INT", "number", false},
		{"bigint", "BIGINT", "number", false},
		{"real", "REAL", "number", false},
		{"float", "FLOAT", "number", false},
		{"double", "DOUBLE", "number", false},
		{"numeric", "NUMERIC", "number", false},
		{"decimal", "DECIMAL", "number", false},
		{"blob fallback", "BLOB", "string", true},
		{"empty fallback", "", "string", true},
		{"unknown fallback", "geography", "string", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotType, gotWarn := m.Map(tc.rawType)
			assert.Equal(t, tc.wantType, gotType)
			if tc.wantWarn {
				assert.NotEmpty(t, gotWarn)
			} else {
				assert.Empty(t, gotWarn)
			}
		})
	}
}

func TestFallbackMapper_TableDriven(t *testing.T) {
	m := NewTypeMapper("unknown_db", nil)
	require.NotNil(t, m)

	gotType, gotWarn := m.Map("anything")
	assert.Equal(t, "string", gotType)
	assert.NotEmpty(t, gotWarn)

	m2 := NewTypeMapper("unknown_db", map[string]string{
		"anything": "number",
	})
	gotType, gotWarn = m2.Map("anything")
	assert.Equal(t, "number", gotType)
	assert.Empty(t, gotWarn)
}

func TestNewTypeMapper_EmptyDBType(t *testing.T) {
	m := NewTypeMapper("", nil)
	require.NotNil(t, m)
	gotType, gotWarn := m.Map("int")
	assert.Equal(t, "string", gotType)
	assert.NotEmpty(t, gotWarn)
}

func TestNormalize(t *testing.T) {
	assert.Equal(t, "int", normalize("INT"))
	assert.Equal(t, "int", normalize("  int  "))
	assert.Equal(t, "int", normalize("int unsigned"))
	assert.Equal(t, "varchar(255)", normalize("VARCHAR(255) UNSIGNED"))
	assert.Equal(t, "", normalize(""))
}
