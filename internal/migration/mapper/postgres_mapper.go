package mapper

type postgresMapper struct {
	baseMapper
}

func (m *postgresMapper) Map(rawType string) (string, string) {
	if override, ok := m.override(rawType); ok {
		return override, ""
	}

	switch {
	case hasPrefix(rawType, "character varying", "varchar", "character", "char"):
		return "string", ""
	case hasPrefix(rawType, "text"):
		return "text", ""
	case hasPrefix(rawType, "integer", "bigint", "smallint", "real", "double precision", "numeric", "decimal"):
		return "number", ""
	case hasPrefix(rawType, "boolean"):
		return "boolean", ""
	case hasPrefix(rawType, "date"):
		return "date", ""
	case hasPrefix(rawType, "timestamp", "timestamp with time zone", "timestamp without time zone", "timestamptz"):
		return "datetime", ""
	case hasPrefix(rawType, "json", "jsonb"):
		return "json", ""
	case hasPrefix(rawType, "array"):
		return "list", ""
	case hasPrefix(rawType, "bytea"):
		return "string", fallbackWarning(rawType)
	case hasPrefix(rawType, "uuid", "inet", "cidr", "macaddr"):
		return "string", ""
	default:
		return "string", fallbackWarning(rawType)
	}
}
