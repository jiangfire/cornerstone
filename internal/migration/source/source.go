package source

import "fmt"

type Source interface {
	Connect(dsn string) error
	Close() error
	ListDatabases() ([]string, error)
	ListTables(dbName string) ([]string, error)
	GetTableSchema(dbName, tableName string) (*TableSchema, error)
	EstimateRowCount(dbName, tableName string) (int64, error)
	QueryRows(dbName, tableName string, opts QueryOptions) ([]map[string]interface{}, error)
	RecommendPaginationStrategy(dbName, tableName string) PaginationStrategy
}

type PaginationStrategy string

const (
	StrategyCursor PaginationStrategy = "cursor"
	StrategyOffset PaginationStrategy = "offset"
)

type QueryOptions struct {
	Strategy     PaginationStrategy
	CursorColumn string
	CursorValue  interface{}
	Offset       int64
	Limit        int64
	Filter       string
}

type TableSchema struct {
	Name        string
	Columns     []ColumnSchema
	PrimaryKey  []string
	UniqueKeys  [][]string
	RowEstimate int64
}

type ColumnSchema struct {
	Name         string
	Type         string
	Nullable     bool
	DefaultValue interface{}
	MaxLength    *int
	Comment      string
	IsPrimaryKey bool
	IsUnique     bool
}

func NewSource(dbType string) (Source, error) {
	switch dbType {
	case "sqlite":
		return &SQLiteSource{}, nil
	case "mysql":
		return &MySQLSource{}, nil
	case "postgres":
		return &PostgresSource{}, nil
	default:
		return nil, fmt.Errorf("unsupported source type: %s", dbType)
	}
}
