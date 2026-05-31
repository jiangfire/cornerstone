package mapper

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMySQLMapper_UsesOverridesAndBuiltins(t *testing.T) {
	m := NewTypeMapper("mysql", map[string]string{
		"json": "text",
	})

	fieldType, warning := m.Map("JSON")
	assert.Equal(t, "text", fieldType)
	assert.Empty(t, warning)

	fieldType, warning = m.Map("tinyint(1)")
	assert.Equal(t, "boolean", fieldType)
	assert.Empty(t, warning)

	fieldType, warning = m.Map("blob")
	assert.Equal(t, "string", fieldType)
	assert.NotEmpty(t, warning)
}

func TestPostgresMapper_MapsKnownTypes(t *testing.T) {
	m := NewTypeMapper("postgres", nil)

	fieldType, warning := m.Map("jsonb")
	assert.Equal(t, "json", fieldType)
	assert.Empty(t, warning)

	fieldType, warning = m.Map("timestamp with time zone")
	assert.Equal(t, "datetime", fieldType)
	assert.Empty(t, warning)
}

func TestSQLiteMapper_FallsBackWithWarning(t *testing.T) {
	m := NewTypeMapper("sqlite", nil)

	fieldType, warning := m.Map("blob")
	assert.Equal(t, "string", fieldType)
	assert.NotEmpty(t, warning)

	fieldType, warning = m.Map("")
	assert.Equal(t, "string", fieldType)
	assert.NotEmpty(t, warning)
}
