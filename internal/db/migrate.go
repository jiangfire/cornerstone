package db

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jiangfire/cornerstone/internal/authz"
	"github.com/jiangfire/cornerstone/internal/config"
	"github.com/jiangfire/cornerstone/internal/models"
	pkgdb "github.com/jiangfire/cornerstone/pkg/db"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type circuitBreaker struct {
	mu        sync.Mutex
	failures  int
	threshold int
	cooldown  time.Duration
	openUntil time.Time
}

func newCircuitBreaker(threshold int, cooldown time.Duration) *circuitBreaker {
	return &circuitBreaker{
		threshold: threshold,
		cooldown:  cooldown,
	}
}

func (cb *circuitBreaker) allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return time.Now().After(cb.openUntil)
}

func (cb *circuitBreaker) markSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
	cb.openUntil = time.Time{}
}

func (cb *circuitBreaker) markFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	if cb.failures >= cb.threshold {
		cb.openUntil = time.Now().Add(cb.cooldown)
	}
}

var (
	tokenCleanupBreaker = newCircuitBreaker(3, 2*time.Minute)
)

// InitDB initializes the database connection
func InitDB(cfg config.DatabaseConfig) error {
	return pkgdb.InitDB(cfg)
}

// CloseDB closes the database connection
func CloseDB() error {
	return pkgdb.CloseDB()
}

// Migrate runs all database migrations
func Migrate() error {
	database := pkgdb.DB()
	logger := zap.L()

	logger.Info("starting database migration...")

	if err := database.AutoMigrate(
		&models.Token{},
		&models.Database{},
		&models.Table{},
		&models.Field{},
		&models.Record{},
		&models.RecordFieldIndex{},
		&models.File{},
	); err != nil {
		return fmt.Errorf("auto migration failed: %w", err)
	}

	logger.Info("schema migration completed")

	if err := createIndexes(database); err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	logger.Info("index creation completed")

	if err := backfillRecordFieldIndexes(database); err != nil {
		return fmt.Errorf("failed to backfill record field indexes: %w", err)
	}

	masterToken := os.Getenv("MASTER_TOKEN")
	if masterToken == "" {
		logger.Warn("MASTER_TOKEN environment variable is not set, Master Token authentication will be unavailable")
	} else {
		logger.Info("MASTER_TOKEN loaded from environment variable")
	}

	logger.Info("database migration completed")
	return nil
}

func createIndexes(db *gorm.DB) error {
	// records list primary path needs to cover table_id + deleted_at + created_at ordering
	if err := createIndexIfNotExists(db, "records", "idx_records_table_deleted_created", "table_id, deleted_at, created_at DESC"); err != nil {
		return err
	}
	if err := createIndexIfNotExists(db, "record_field_indexes", "idx_rfi_text_lookup", "table_id, field_id, value_text, deleted_at, created_at DESC, record_id"); err != nil {
		return err
	}
	if err := createIndexIfNotExists(db, "record_field_indexes", "idx_rfi_number_lookup", "table_id, field_id, value_number, deleted_at, created_at DESC, record_id"); err != nil {
		return err
	}
	if err := createIndexIfNotExists(db, "record_field_indexes", "idx_rfi_bool_lookup", "table_id, field_id, value_bool, deleted_at, created_at DESC, record_id"); err != nil {
		return err
	}

	// PostgreSQL: GIN index to accelerate JSONB queries (data @>, JSON path, etc.)
	if isPostgres(db) {
		if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_records_data_gin ON records USING GIN (data)").Error; err != nil {
			return err
		}
	}

	return nil
}

const recordFieldIndexBackfillBatchSize = 500
const recordFieldIndexTextMaxLength = 512

func backfillRecordFieldIndexes(db *gorm.DB) error {
	var fields []models.Field
	if err := db.Where("deleted_at IS NULL").Find(&fields).Error; err != nil {
		return err
	}
	if len(fields) == 0 {
		return nil
	}

	fieldsByTable := make(map[string][]models.Field, len(fields))
	for _, field := range fields {
		fieldsByTable[field.TableID] = append(fieldsByTable[field.TableID], field)
	}

	return db.Model(&models.Record{}).
		Where("deleted_at IS NULL").
		FindInBatches(&[]models.Record{}, recordFieldIndexBackfillBatchSize, func(tx *gorm.DB, _ int) error {
			records, ok := tx.Statement.Dest.(*[]models.Record)
			if !ok || len(*records) == 0 {
				return nil
			}

			recordIDs := make([]string, 0, len(*records))
			for _, record := range *records {
				recordIDs = append(recordIDs, record.ID)
			}

			var existing []string
			if err := tx.Model(&models.RecordFieldIndex{}).
				Where("record_id IN ? AND deleted_at IS NULL", recordIDs).
				Distinct("record_id").
				Pluck("record_id", &existing).Error; err != nil {
				return err
			}
			hasIndex := make(map[string]struct{}, len(existing))
			for _, recordID := range existing {
				hasIndex[recordID] = struct{}{}
			}

			rows := make([]models.RecordFieldIndex, 0, len(*records)*4)
			for _, record := range *records {
				if _, ok := hasIndex[record.ID]; ok {
					continue
				}
				payload := make(map[string]interface{})
				if err := json.Unmarshal([]byte(record.Data), &payload); err != nil {
					continue
				}
				rows = append(rows, buildBackfillRecordFieldIndexRows(record, fieldsByTable[record.TableID], payload)...)
			}
			if len(rows) == 0 {
				return nil
			}
			return tx.CreateInBatches(&rows, 1000).Error
		}).Error
}

func buildBackfillRecordFieldIndexRows(record models.Record, fields []models.Field, payload map[string]interface{}) []models.RecordFieldIndex {
	rows := make([]models.RecordFieldIndex, 0, len(fields))
	for _, field := range fields {
		value, ok := payload[field.Name]
		if !ok || value == nil {
			continue
		}
		row, ok := buildBackfillRecordFieldIndexRow(record, field, value)
		if ok {
			rows = append(rows, row)
		}
	}
	return rows
}

func buildBackfillRecordFieldIndexRow(record models.Record, field models.Field, value interface{}) (models.RecordFieldIndex, bool) {
	row := models.RecordFieldIndex{
		TableID:   record.TableID,
		RecordID:  record.ID,
		FieldID:   field.ID,
		FieldName: field.Name,
	}

	switch strings.ToLower(strings.TrimSpace(field.Type)) {
	case "string", "text", "date", "datetime":
		text, ok := value.(string)
		if !ok || len(text) > recordFieldIndexTextMaxLength {
			return models.RecordFieldIndex{}, false
		}
		row.ValueType = "text"
		row.ValueText = text
		return row, true
	case "number":
		number, ok := backfillRecordFieldIndexNumber(value)
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
		text, ok := backfillRecordFieldIndexJSONText(value)
		if !ok {
			return models.RecordFieldIndex{}, false
		}
		row.ValueType = "text"
		row.ValueText = text
		return row, true
	default:
		return models.RecordFieldIndex{}, false
	}
}

func backfillRecordFieldIndexNumber(value interface{}) (float64, bool) {
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

func backfillRecordFieldIndexJSONText(value interface{}) (string, bool) {
	if text, ok := value.(string); ok {
		if len(text) > recordFieldIndexTextMaxLength {
			return "", false
		}
		return text, true
	}
	encoded, err := json.Marshal(value)
	if err != nil || len(encoded) > recordFieldIndexTextMaxLength {
		return "", false
	}
	return string(encoded), true
}

// createIndexIfNotExists creates indexes in a cross-database compatible way
func createIndexIfNotExists(db *gorm.DB, table, indexName, column string) error {
	exists, err := indexExists(db, table, indexName)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	sql := fmt.Sprintf("CREATE INDEX %s ON %s(%s)", indexName, table, column)
	return db.Exec(sql).Error
}

// indexExists checks if an index already exists
func indexExists(db *gorm.DB, table, indexName string) (bool, error) {
	var count int64
	switch db.Name() {
	case "sqlite":
		// SQLite: query sqlite_master
		if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?", indexName).Scan(&count).Error; err != nil {
			return false, err
		}
	case "postgres":
		// PostgreSQL: query pg_indexes
		if err := db.Raw("SELECT COUNT(*) FROM pg_indexes WHERE indexname=?", indexName).Scan(&count).Error; err != nil {
			return false, err
		}
	case "mysql":
		// MySQL: query information_schema.STATISTICS
		if err := db.Raw("SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND INDEX_NAME = ?", table, indexName).Scan(&count).Error; err != nil {
			return false, err
		}
	default:
		return false, fmt.Errorf("unsupported database type: %s", db.Name())
	}
	return count > 0, nil
}

func isPostgres(db *gorm.DB) bool {
	return db.Name() == "postgres"
}

// CleanupExpiredTokens cleans up expired tokens
func CleanupExpiredTokens() error {
	database := pkgdb.DB()
	logger := zap.L()

	result := database.Where("expires_at IS NOT NULL AND expires_at <= ?", time.Now()).Delete(&models.Token{})
	if result.Error != nil {
		return fmt.Errorf("failed to cleanup expired tokens: %w", result.Error)
	}

	if result.RowsAffected > 0 {
		logger.Info("cleaned up expired tokens", zap.Int64("count", result.RowsAffected))
		authz.ClearTokenCache()
	}
	return nil
}

// SetupPeriodicTasks sets up periodic tasks
func SetupPeriodicTasks(ctx context.Context) *sync.WaitGroup {
	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := runProtectedTask("cleanup expired tokens", tokenCleanupBreaker, CleanupExpiredTokens); err != nil {
					zap.L().Error("scheduled cleanup of expired tokens failed", zap.Error(err))
				}
			}
		}
	}()

	return wg
}

func runProtectedTask(name string, breaker *circuitBreaker, task func() error) error {
	if !breaker.allow() {
		zap.L().Warn("task circuit breaker open, skipping execution", zap.String("task", name))
		return nil
	}

	err := retry(task, 3, 500*time.Millisecond)
	if err != nil {
		breaker.markFailure()
		return err
	}

	breaker.markSuccess()
	return nil
}

func retry(task func() error, maxAttempts int, baseDelay time.Duration) error {
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		if err := task(); err != nil {
			lastErr = err
			if i < maxAttempts-1 {
				time.Sleep(baseDelay * time.Duration(i+1))
			}
			continue
		}
		return nil
	}
	return lastErr
}
