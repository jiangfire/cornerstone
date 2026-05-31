package mapper

import (
	"fmt"
	"strings"
)

type TypeMapper interface {
	Map(rawType string) (cornerstoneType string, warning string)
}

func NewTypeMapper(dbType string, overrides map[string]string) TypeMapper {
	base := newBaseMapper(overrides)
	switch normalize(rawTypeOrType(dbType)) {
	case "mysql":
		return &mysqlMapper{baseMapper: base}
	case "postgres":
		return &postgresMapper{baseMapper: base}
	case "sqlite":
		return &sqliteMapper{baseMapper: base}
	default:
		return &fallbackMapper{baseMapper: base}
	}
}

type baseMapper struct {
	overrides map[string]string
}

func newBaseMapper(overrides map[string]string) baseMapper {
	normalized := make(map[string]string, len(overrides))
	for key, value := range overrides {
		normalized[normalize(key)] = value
	}
	return baseMapper{overrides: normalized}
}

func (b baseMapper) override(rawType string) (string, bool) {
	value, ok := b.overrides[normalize(rawType)]
	return value, ok
}

func normalize(rawType string) string {
	value := strings.ToLower(strings.TrimSpace(rawType))
	value = strings.ReplaceAll(value, "unsigned", "")
	return strings.Join(strings.Fields(value), " ")
}

func hasPrefix(rawType string, prefixes ...string) bool {
	value := normalize(rawType)
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, normalize(prefix)) {
			return true
		}
	}
	return false
}

func rawTypeOrType(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unknown"
	}
	return value
}

func fallbackWarning(rawType string) string {
	return fmt.Sprintf("column type %q fell back to string", rawType)
}

type fallbackMapper struct {
	baseMapper
}

func (m *fallbackMapper) Map(rawType string) (string, string) {
	if override, ok := m.override(rawType); ok {
		return override, ""
	}
	return "string", fallbackWarning(rawType)
}
