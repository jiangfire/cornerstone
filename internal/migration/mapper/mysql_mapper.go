package mapper

type mysqlMapper struct {
	baseMapper
}

func (m *mysqlMapper) Map(rawType string) (string, string) {
	if override, ok := m.override(rawType); ok {
		return override, ""
	}

	switch {
	case hasPrefix(rawType, "tinyint(1)"):
		return "boolean", ""
	case hasPrefix(rawType, "varchar", "char", "tinytext"):
		return "string", ""
	case hasPrefix(rawType, "text", "mediumtext", "longtext"):
		return "text", ""
	case hasPrefix(rawType, "int", "bigint", "smallint", "float", "double", "decimal", "numeric"):
		return "number", ""
	case hasPrefix(rawType, "date"):
		return "date", ""
	case hasPrefix(rawType, "datetime", "timestamp"):
		return "datetime", ""
	case hasPrefix(rawType, "json"):
		return "json", ""
	case hasPrefix(rawType, "enum", "set"):
		return "list", ""
	case hasPrefix(rawType, "blob", "binary", "varbinary"):
		return "string", fallbackWarning(rawType)
	default:
		return "string", fallbackWarning(rawType)
	}
}
