package mapper

import "strings"

type sqliteMapper struct {
	baseMapper
}

func (m *sqliteMapper) Map(rawType string) (string, string) {
	if override, ok := m.override(rawType); ok {
		return override, ""
	}

	value := normalize(rawType)
	switch {
	case value == "":
		return "string", fallbackWarning(rawType)
	case strings.Contains(value, "tinyint(1)") || strings.Contains(value, "bool"):
		return "boolean", ""
	case strings.Contains(value, "text") || strings.Contains(value, "char") || strings.Contains(value, "clob"):
		return "string", ""
	case strings.Contains(value, "json"):
		return "json", ""
	case strings.Contains(value, "int"):
		return "number", ""
	case strings.Contains(value, "real") || strings.Contains(value, "floa") || strings.Contains(value, "doub") || strings.Contains(value, "numeric") || strings.Contains(value, "decimal"):
		return "number", ""
	case strings.Contains(value, "blob"):
		return "string", fallbackWarning(rawType)
	default:
		return "string", fallbackWarning(rawType)
	}
}
