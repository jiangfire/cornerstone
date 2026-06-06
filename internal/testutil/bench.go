package testutil

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jiangfire/cornerstone/internal/config"
	internaldb "github.com/jiangfire/cornerstone/internal/db"
	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/pkg/cache"
	pkgdb "github.com/jiangfire/cornerstone/pkg/db"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type BenchmarkSeedConfig struct {
	RecordCount          int
	ExtraFieldCount      int
	NoiseTableCount      int
	NoiseRecordsPerTable int
}

type BenchmarkFixture struct {
	DB          *gorm.DB
	DBType      string
	DBPath      string
	Database    *models.Database
	Table       *models.Table
	NoiseTables []*models.Table
	Fields      []models.Field
	MasterToken *models.Token
	ScopedToken *models.Token
}

const benchmarkRecordFieldIndexTextLength = 512

func resolveBenchmarkDatabaseConfig(tb testing.TB) (config.DatabaseConfig, error) {
	tb.Helper()

	dbType := strings.TrimSpace(os.Getenv("DB_TYPE"))
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dbType == "" {
		dbType = "sqlite"
	}
	if dbType == "sqlite" && databaseURL == "" {
		databaseURL = filepath.Join(tb.TempDir(), "cornerstone-bench.sqlite")
	}

	cfg := config.DatabaseConfig{
		Type:        dbType,
		URL:         databaseURL,
		MaxOpen:     4,
		MaxIdle:     4,
		MaxLifetime: 3600,
	}
	if err := (&config.Config{Database: cfg, Server: config.ServerConfig{Port: "8080"}}).Validate(); err != nil {
		return config.DatabaseConfig{}, err
	}
	return cfg, nil
}

func SetupBenchmarkFixture(tb testing.TB, cfg BenchmarkSeedConfig) *BenchmarkFixture {
	tb.Helper()

	if cfg.RecordCount <= 0 {
		cfg.RecordCount = 2000
	}
	if cfg.NoiseRecordsPerTable < 0 {
		cfg.NoiseRecordsPerTable = 0
	}
	if cfg.NoiseTableCount < 0 {
		cfg.NoiseTableCount = 0
	}

	dbCfg, err := resolveBenchmarkDatabaseConfig(tb)
	require.NoError(tb, err)
	require.NoError(tb, pkgdb.CloseDB())
	require.NoError(tb, internaldb.InitDB(dbCfg))

	database := pkgdb.DB()
	require.NoError(tb, internaldb.Migrate())
	cleanupTables(database, tb)
	cache.ClearAll()

	tb.Cleanup(func() {
		cleanupTables(database, tb)
		cache.ClearAll()
		if sqlDB, err := database.DB(); err == nil {
			_ = sqlDB.Close()
		}
		pkgdb.SetDB(nil)
	})

	fixture := &BenchmarkFixture{
		DB:     database,
		DBType: database.Name(),
		DBPath: dbCfg.URL,
	}

	fixture.seed(tb, cfg)
	return fixture
}

func SetupSQLiteBenchmarkFixture(tb testing.TB, cfg BenchmarkSeedConfig) *BenchmarkFixture {
	tb.Helper()
	tb.Setenv("DB_TYPE", "sqlite")
	tb.Setenv("DATABASE_URL", filepath.Join(tb.TempDir(), "cornerstone-bench.sqlite"))
	return SetupBenchmarkFixture(tb, cfg)
}

func (f *BenchmarkFixture) seed(tb testing.TB, cfg BenchmarkSeedConfig) {
	tb.Helper()

	tokenSuffix := fmt.Sprintf("%s_%d", sanitizeBenchmarkIdentifier(tb.Name()), time.Now().UnixNano())
	master := &models.Token{
		Name:     "bench-master",
		Token:    "bench_master_" + tokenSuffix,
		IsMaster: true,
		Scopes:   "{}",
	}
	require.NoError(tb, f.DB.Create(master).Error)

	database := &models.Database{
		Name:        "bench_db",
		Description: "benchmark dataset",
	}
	require.NoError(tb, f.DB.Create(database).Error)

	table := &models.Table{
		DatabaseID:  database.ID,
		Name:        "bench_records",
		Description: "benchmark records",
	}
	require.NoError(tb, f.DB.Create(table).Error)

	fields := []models.Field{
		{TableID: table.ID, Name: "name", Type: "string"},
		{TableID: table.ID, Name: "status", Type: "string"},
		{TableID: table.ID, Name: "category", Type: "string"},
		{TableID: table.ID, Name: "score", Type: "number"},
		{TableID: table.ID, Name: "payload", Type: "json"},
	}

	for i := 0; i < cfg.ExtraFieldCount; i++ {
		fields = append(fields, models.Field{
			TableID: table.ID,
			Name:    fmt.Sprintf("extra_%02d", i),
			Type:    "string",
		})
	}

	require.NoError(tb, f.DB.Create(&fields).Error)

	noiseTables := make([]*models.Table, 0, cfg.NoiseTableCount)
	for i := 0; i < cfg.NoiseTableCount; i++ {
		noiseTable := &models.Table{
			DatabaseID:  database.ID,
			Name:        fmt.Sprintf("bench_records_noise_%02d", i),
			Description: "benchmark noise records",
		}
		require.NoError(tb, f.DB.Create(noiseTable).Error)

		noiseFields := cloneBenchmarkFieldsForTable(fields, noiseTable.ID)
		require.NoError(tb, f.DB.Create(&noiseFields).Error)
		noiseTables = append(noiseTables, noiseTable)
	}

	scopeJSON := fmt.Sprintf(
		`{"databases":{%q:"viewer"},"tables":{%q:{"role":"viewer"}}}`,
		database.ID,
		table.ID,
	)
	scoped := &models.Token{
		Name:     "bench-scoped",
		Token:    "bench_scoped_" + tokenSuffix,
		IsMaster: false,
		Scopes:   scopeJSON,
	}
	require.NoError(tb, f.DB.Create(scoped).Error)

	records := make([]models.Record, 0, cfg.RecordCount)
	statuses := []string{"new", "paid", "archived", "shipped"}
	categories := []string{"alpha", "beta", "gamma", "delta", "omega"}

	for i := 0; i < cfg.RecordCount; i++ {
		payload := map[string]any{
			"name":     fmt.Sprintf("user-%06d", i),
			"status":   statuses[i%len(statuses)],
			"category": categories[i%len(categories)],
			"score":    i % 1000,
			"payload": map[string]any{
				"index": i,
				"flags": []string{"a", "b", "c"},
			},
		}
		for extra := 0; extra < cfg.ExtraFieldCount; extra++ {
			payload[fmt.Sprintf("extra_%02d", extra)] = fmt.Sprintf("value-%d-%d", extra, i%17)
		}

		dataJSON, err := json.Marshal(payload)
		require.NoError(tb, err)

		records = append(records, models.Record{
			TableID: table.ID,
			Data:    models.JSONField(dataJSON),
			Version: 1,
		})
	}

	require.NoError(tb, f.DB.CreateInBatches(&records, 500).Error)
	seedBenchmarkRecordFieldIndexes(tb, f.DB, table.ID, fields, records)

	if cfg.NoiseTableCount > 0 && cfg.NoiseRecordsPerTable > 0 {
		noiseRecords := make([]models.Record, 0, cfg.NoiseTableCount*cfg.NoiseRecordsPerTable)
		for tableIdx, noiseTable := range noiseTables {
			for i := 0; i < cfg.NoiseRecordsPerTable; i++ {
				globalIndex := tableIdx*cfg.NoiseRecordsPerTable + i
				payload := map[string]any{
					"name":     fmt.Sprintf("noise-user-%02d-%06d", tableIdx, i),
					"status":   statuses[globalIndex%len(statuses)],
					"category": categories[globalIndex%len(categories)],
					"score":    globalIndex % 1000,
					"payload": map[string]any{
						"index": globalIndex,
						"flags": []string{"x", "y", "z"},
					},
				}
				for extra := 0; extra < cfg.ExtraFieldCount; extra++ {
					payload[fmt.Sprintf("extra_%02d", extra)] = fmt.Sprintf("noise-value-%d-%d", extra, globalIndex%23)
				}

				dataJSON, err := json.Marshal(payload)
				require.NoError(tb, err)

				noiseRecords = append(noiseRecords, models.Record{
					TableID: noiseTable.ID,
					Data:    models.JSONField(dataJSON),
					Version: 1,
				})
			}
		}

		require.NoError(tb, f.DB.CreateInBatches(&noiseRecords, 500).Error)
		fieldsByTable := make(map[string][]models.Field, len(noiseTables))
		for _, noiseTable := range noiseTables {
			var noiseFields []models.Field
			require.NoError(tb, f.DB.Where("table_id = ? AND deleted_at IS NULL", noiseTable.ID).
				Order("created_at ASC").
				Find(&noiseFields).Error)
			fieldsByTable[noiseTable.ID] = noiseFields
		}
		recordsByTable := make(map[string][]models.Record, len(noiseTables))
		for _, record := range noiseRecords {
			recordsByTable[record.TableID] = append(recordsByTable[record.TableID], record)
		}
		for tableID, tableRecords := range recordsByTable {
			seedBenchmarkRecordFieldIndexes(tb, f.DB, tableID, fieldsByTable[tableID], tableRecords)
		}
	}
	cache.ClearAll()

	f.Database = database
	f.Table = table
	f.NoiseTables = noiseTables
	f.Fields = fields
	f.MasterToken = master
	f.ScopedToken = scoped
}

func seedBenchmarkRecordFieldIndexes(tb testing.TB, database *gorm.DB, tableID string, fields []models.Field, records []models.Record) {
	tb.Helper()
	if len(records) == 0 || len(fields) == 0 {
		return
	}

	fieldByName := make(map[string]models.Field, len(fields))
	for _, field := range fields {
		fieldByName[field.Name] = field
	}

	indexRows := make([]models.RecordFieldIndex, 0, len(records)*len(fields))
	for _, record := range records {
		payload := make(map[string]interface{})
		require.NoError(tb, json.Unmarshal([]byte(record.Data), &payload))
		for fieldName, value := range payload {
			field, ok := fieldByName[fieldName]
			if !ok || value == nil {
				continue
			}
			row, ok := benchmarkRecordFieldIndexRow(tableID, record.ID, field, value)
			if ok {
				indexRows = append(indexRows, row)
			}
		}
	}
	if len(indexRows) > 0 {
		require.NoError(tb, database.CreateInBatches(&indexRows, 1000).Error)
	}
}

func benchmarkRecordFieldIndexRow(tableID, recordID string, field models.Field, value interface{}) (models.RecordFieldIndex, bool) {
	row := models.RecordFieldIndex{
		TableID:   tableID,
		RecordID:  recordID,
		FieldID:   field.ID,
		FieldName: field.Name,
	}

	switch field.Type {
	case "string", "text", "date", "datetime":
		text, ok := value.(string)
		if !ok || len(text) > benchmarkRecordFieldIndexTextLength {
			return models.RecordFieldIndex{}, false
		}
		row.ValueType = "text"
		row.ValueText = text
		return row, true
	case "number":
		number, ok := benchmarkRecordFieldIndexNumber(value)
		if !ok {
			return models.RecordFieldIndex{}, false
		}
		row.ValueType = "number"
		row.ValueNumber = &number
		return row, true
	case "boolean":
		boolean, ok := value.(bool)
		if !ok {
			return models.RecordFieldIndex{}, false
		}
		row.ValueType = "bool"
		row.ValueBool = &boolean
		return row, true
	case "json":
		if text, ok := value.(string); ok {
			if len(text) > benchmarkRecordFieldIndexTextLength {
				return models.RecordFieldIndex{}, false
			}
			row.ValueType = "text"
			row.ValueText = text
			return row, true
		}
		encoded, err := json.Marshal(value)
		if err != nil || len(encoded) > benchmarkRecordFieldIndexTextLength {
			return models.RecordFieldIndex{}, false
		}
		row.ValueType = "text"
		row.ValueText = string(encoded)
		return row, true
	default:
		return models.RecordFieldIndex{}, false
	}
}

func benchmarkRecordFieldIndexNumber(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}

func cloneBenchmarkFieldsForTable(fields []models.Field, tableID string) []models.Field {
	cloned := make([]models.Field, 0, len(fields))
	for _, field := range fields {
		cloned = append(cloned, models.Field{
			TableID:     tableID,
			Name:        field.Name,
			Type:        field.Type,
			Description: field.Description,
			Required:    field.Required,
			Options:     field.Options,
		})
	}
	return cloned
}

func sanitizeBenchmarkIdentifier(value string) string {
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}
