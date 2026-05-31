package migration

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jiangfire/cornerstone/internal/migration/mapper"
	"github.com/jiangfire/cornerstone/internal/migration/source"
	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/internal/services"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type RunnerOptions struct {
	StateDir      string
	ResumeID      string
	MigrationID   string
	SourceFactory func() (source.Source, error)
}

type Runner struct {
	db            *gorm.DB
	masterToken   string
	cfg           Config
	opts          RunnerOptions
	store         *StateStore
	src           source.Source
	sourceFactory func() (source.Source, error)
	mapper        mapper.TypeMapper
	migrationID   string

	state   MigrationState
	stateMu sync.Mutex
}

type tableExecutionResult struct {
	report MigrationTableReport
	err    error
}

func NewRunner(db *gorm.DB, masterToken string, cfg Config, opts RunnerOptions) (*Runner, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	migrationID := opts.MigrationID
	if migrationID == "" {
		if opts.ResumeID != "" {
			migrationID = opts.ResumeID
		} else {
			migrationID = "mig_" + time.Now().UTC().Format("20060102_150405")
		}
	}

	sourceFactory := opts.SourceFactory
	if sourceFactory == nil {
		sourceFactory = func() (source.Source, error) {
			src, err := source.NewSource(normalizeLower(cfg.Source.Type))
			if err != nil {
				return nil, newMigrationError(ErrCodeUnsupportedSource, "不支持的源数据库类型", err)
			}
			return src, nil
		}
	}

	return &Runner{
		db:            db,
		masterToken:   masterToken,
		cfg:           cfg,
		opts:          opts,
		store:         NewStateStore(opts.StateDir),
		sourceFactory: sourceFactory,
		mapper:        mapper.NewTypeMapper(cfg.Source.Type, cfg.Mapping.Overrides),
		migrationID:   migrationID,
	}, nil
}

func (r *Runner) Preview() (*PreviewPlan, error) {
	if err := r.ensureSource(); err != nil {
		return nil, err
	}
	defer r.closeSource()

	tables, err := r.filteredTables()
	if err != nil {
		return nil, err
	}

	plan := &PreviewPlan{
		Source: PreviewSource{
			Type:     normalizeLower(r.cfg.Source.Type),
			Database: r.sourceDatabaseName(),
		},
		TargetDatabase: r.cfg.EffectiveTargetDatabase(),
		Tables:         make([]PreviewTablePlan, 0, len(tables)),
	}

	for _, tableName := range tables {
		schema, err := r.src.GetTableSchema("", tableName)
		if err != nil {
			return nil, err
		}
		planTable := r.buildPreviewTablePlan(tableName, schema)
		plan.TotalEstimatedRows += schema.RowEstimate
		plan.Tables = append(plan.Tables, planTable)
	}

	return plan, nil
}

func (r *Runner) Run() (*MigrationReport, error) {
	if r.db == nil {
		return nil, errors.New("target db is required")
	}
	if err := r.ensureSource(); err != nil {
		return nil, err
	}
	defer r.closeSource()

	plan, err := r.buildPlan()
	if err != nil {
		return nil, err
	}

	state, err := r.loadOrInitState(plan)
	if err != nil {
		return nil, err
	}

	zap.L().Info("开始迁移",
		zap.String("migration_id", r.migrationID),
		zap.String("source_type", r.cfg.Source.Type),
		zap.String("target_database", plan.TargetDatabase),
		zap.Int("tables", len(plan.Tables)),
	)

	report := &MigrationReport{
		Status:    StatusCompleted,
		StartedAt: state.StartedAt,
		Summary: ReportSummary{
			TablesTotal:  len(plan.Tables),
			RecordsTotal: plan.TotalEstimatedRows,
		},
		Validation: ValidationReport{
			Status:  ValidationPassed,
			Details: []ValidationTableDetail{},
		},
	}

	targetDB, err := r.ensureTargetDatabase()
	if err != nil {
		return nil, newMigrationError(ErrCodeTargetCreate, "创建目标数据库失败", err)
	}

	results := r.executeTablePlans(targetDB.ID, plan.Tables)
	for _, result := range results {
		report.Tables = append(report.Tables, result.report)
		if result.err != nil {
			report.Status = StatusCompletedWithIssues
			report.Summary.TablesFailed++
			if !r.cfg.Options.ContinueOnError {
				report.Status = StatusFailed
			}
			continue
		}
		report.Summary.TablesSuccess++
		report.Summary.RecordsInserted += result.report.RecordsInserted
	}

	if r.cfg.Options.ValidateAfter {
		report.Validation, err = r.validateMigration(targetDB.ID, plan.Tables)
		if err != nil {
			return nil, err
		}
		if report.Validation.TablesFailed > 0 || report.Validation.TablesWarnings > 0 {
			report.Status = StatusCompletedWithIssues
		}
	} else {
		report.Validation = r.finalizeValidation(report.Tables)
	}

	report.FinishedAt = time.Now().UTC()
	zap.L().Info("迁移完成",
		zap.String("migration_id", r.migrationID),
		zap.String("status", report.Status),
		zap.Int("tables_success", report.Summary.TablesSuccess),
		zap.Int("tables_failed", report.Summary.TablesFailed),
		zap.Int64("records_inserted", report.Summary.RecordsInserted),
	)
	return report, nil
}

type compiledPlan struct {
	PreviewPlan
	tableSchemas map[string]*source.TableSchema
}

func (r *Runner) buildPlan() (*compiledPlan, error) {
	tables, err := r.filteredTables()
	if err != nil {
		return nil, err
	}

	plan := &compiledPlan{
		PreviewPlan: PreviewPlan{
			Source: PreviewSource{
				Type:     normalizeLower(r.cfg.Source.Type),
				Database: r.sourceDatabaseName(),
			},
			TargetDatabase: r.cfg.EffectiveTargetDatabase(),
			Tables:         make([]PreviewTablePlan, 0, len(tables)),
		},
		tableSchemas: map[string]*source.TableSchema{},
	}

	for _, tableName := range tables {
		schema, err := r.src.GetTableSchema("", tableName)
		if err != nil {
			return nil, err
		}
		plan.tableSchemas[tableName] = schema
		rowPlan := r.buildPreviewTablePlan(tableName, schema)
		plan.TotalEstimatedRows += schema.RowEstimate
		plan.Tables = append(plan.Tables, rowPlan)
	}
	return plan, nil
}

func (r *Runner) buildPreviewTablePlan(tableName string, schema *source.TableSchema) PreviewTablePlan {
	planTable := PreviewTablePlan{
		SourceTable:       tableName,
		TargetTable:       r.targetTableName(tableName),
		Fields:            len(schema.Columns),
		EstimatedRows:     schema.RowEstimate,
		MigrationStrategy: string(r.pickStrategyWithSource(r.src, tableName, schema)),
	}
	for _, column := range schema.Columns {
		_, warning := r.mapper.Map(column.Type)
		if warning != "" {
			planTable.TypeMappingWarnings = append(planTable.TypeMappingWarnings, fmt.Sprintf("%s column %q (%s): %s", ErrCodeTypeMappingWarn, column.Name, column.Type, warning))
		}
	}
	if planTable.MigrationStrategy == string(source.StrategyOffset) {
		planTable.TypeMappingWarnings = append(planTable.TypeMappingWarnings, fmt.Sprintf("%s 表 %q 缺少可用游标列，回退到 OFFSET 分页", ErrCodeOffsetFallbackWarn, tableName))
	}
	return planTable
}

func (r *Runner) filteredTables() ([]string, error) {
	tables, err := r.src.ListTables("")
	if err != nil {
		return nil, err
	}

	includeSet := make(map[string]bool, len(r.cfg.Tables.Include))
	for _, table := range r.cfg.Tables.Include {
		includeSet[table] = true
	}
	excludeSet := make(map[string]bool, len(r.cfg.Tables.Exclude))
	for _, table := range r.cfg.Tables.Exclude {
		excludeSet[table] = true
	}

	result := make([]string, 0, len(tables))
	for _, table := range tables {
		if len(includeSet) > 0 && !includeSet[table] {
			continue
		}
		if excludeSet[table] {
			continue
		}
		result = append(result, table)
	}
	return result, nil
}

func (r *Runner) ensureTargetDatabase() (*models.Database, error) {
	name := r.cfg.EffectiveTargetDatabase()
	var existing models.Database
	err := r.db.Where("name = ? AND deleted_at IS NULL", name).First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	created, err := services.NewDatabaseService(r.db).CreateDatabase(services.CreateDBRequest{Name: name}, r.masterToken)
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (r *Runner) runTable(databaseID string, tablePlan PreviewTablePlan) (MigrationTableReport, error) {
	schema, err := r.src.GetTableSchema("", tablePlan.SourceTable)
	if err != nil {
		return MigrationTableReport{Source: tablePlan.SourceTable, Target: tablePlan.TargetTable, Status: TableStatusFailed}, newMigrationError(ErrCodeTableSchema, "读取源表结构失败", err)
	}

	targetTable, fieldsCreated, err := r.ensureTargetTable(databaseID, tablePlan, schema)
	if err != nil {
		return MigrationTableReport{Source: tablePlan.SourceTable, Target: tablePlan.TargetTable, Status: TableStatusFailed}, newMigrationError(ErrCodeTableSchema, "创建目标表结构失败", err)
	}

	tableState := r.getTableState(tablePlan.SourceTable)
	tableState.Status = TableStatusInProgress
	if tableState.CursorColumn == "" {
		tableState.CursorColumn = r.resolveCursorColumn(schema)
	}
	tableState.TotalEstimate = tablePlan.EstimatedRows
	if err := r.saveTableState(tablePlan.SourceTable, tableState); err != nil {
		return MigrationTableReport{Source: tablePlan.SourceTable, Target: tablePlan.TargetTable, Status: TableStatusFailed}, err
	}

	if r.cfg.Data.Enabled {
		if err := r.importTableData(targetTable.ID, tablePlan.SourceTable, schema, &tableState); err != nil {
			if r.cfg.Options.RollbackOnFailure == RollbackTable {
				_ = r.rollbackTable(targetTable.ID)
			}
			tableState.Status = TableStatusFailed
			_ = r.saveTableState(tablePlan.SourceTable, tableState)
			zap.L().Error("表数据迁移失败",
				zap.String("migration_id", r.migrationID),
				zap.String("table", tablePlan.SourceTable),
				zap.Error(err),
			)
			return MigrationTableReport{
				Source:        tablePlan.SourceTable,
				Target:        tablePlan.TargetTable,
				Status:        TableStatusFailed,
				FieldsCreated: fieldsCreated,
				Error:         err.Error(),
				Warnings:      tablePlan.TypeMappingWarnings,
			}, err
		}
	}

	tableState.Status = TableStatusCompleted
	if err := r.saveTableState(tablePlan.SourceTable, tableState); err != nil {
		return MigrationTableReport{Source: tablePlan.SourceTable, Target: tablePlan.TargetTable, Status: TableStatusFailed}, err
	}

	report, err := r.validationAndReportForExisting(databaseID, tablePlan)
	if err != nil {
		return MigrationTableReport{Source: tablePlan.SourceTable, Target: tablePlan.TargetTable, Status: TableStatusFailed}, err
	}
	report.FieldsCreated = fieldsCreated
	zap.L().Info("表迁移完成",
		zap.String("migration_id", r.migrationID),
		zap.String("table", tablePlan.SourceTable),
		zap.Int64("records", report.RecordsInserted),
	)
	return report, nil
}

func (r *Runner) ensureTargetTable(databaseID string, tablePlan PreviewTablePlan, schema *source.TableSchema) (*models.Table, int, error) {
	var table models.Table
	err := r.db.Where("database_id = ? AND name = ? AND deleted_at IS NULL", databaseID, tablePlan.TargetTable).First(&table).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		created, createErr := services.NewTableService(r.db).CreateTable(services.CreateTableRequest{
			DatabaseID: databaseID,
			Name:       tablePlan.TargetTable,
		}, r.masterToken)
		if createErr != nil {
			return nil, 0, createErr
		}
		table = *created
	} else if err != nil {
		return nil, 0, err
	}

	var existingFields []models.Field
	if err := r.db.Where("table_id = ? AND deleted_at IS NULL", table.ID).Find(&existingFields).Error; err != nil {
		return nil, 0, err
	}
	existingByName := make(map[string]models.Field, len(existingFields))
	for _, field := range existingFields {
		existingByName[field.Name] = field
	}

	fieldSvc := services.NewFieldService(r.db)
	createdCount := 0
	for _, column := range schema.Columns {
		if _, ok := existingByName[column.Name]; ok {
			continue
		}
		fieldType, _ := r.mapper.Map(column.Type)
		if _, err := fieldSvc.CreateField(services.CreateFieldRequest{
			TableID:  table.ID,
			Name:     column.Name,
			Type:     fieldType,
			Required: !column.Nullable,
		}, r.masterToken); err != nil {
			return nil, createdCount, err
		}
		createdCount++
	}
	return &table, createdCount, nil
}

func (r *Runner) importTableData(targetTableID, sourceTable string, schema *source.TableSchema, state *TableState) error {
	strategy := r.pickStrategyWithSource(r.src, sourceTable, schema)
	cursorColumn := state.CursorColumn
	offset := state.ProcessedCount

	for {
		cursorValue := normalizeCursorValue(state.CursorValue)
		rows, err := r.src.QueryRows("", sourceTable, source.QueryOptions{
			Strategy:     strategy,
			CursorColumn: cursorColumn,
			CursorValue:  cursorValue,
			Offset:       offset,
			Limit:        int64(r.cfg.Data.BatchSize),
			Filter:       r.cfg.Data.Filters[sourceTable],
		})
		if err != nil {
			return newMigrationError(ErrCodeTableData, "读取源表数据失败", err)
		}
		if len(rows) == 0 {
			return nil
		}

		batchPayloads := make([]string, 0, len(rows))
		insertedCount := int64(0)
		for _, row := range rows {
			if cursorColumn != "" {
				exists, err := r.recordExists(targetTableID, cursorColumn, row[cursorColumn])
				if err != nil {
					return newMigrationError(ErrCodeTableData, "检查断点续传重复记录失败", err)
				}
				if exists {
					state.ProcessedCount++
					state.CursorValue = row[cursorColumn]
					if strategy == source.StrategyOffset {
						offset++
					}
					continue
				}
			}

			payload := r.normalizeRow(schema, row)
			payloadJSON, err := json.Marshal(payload)
			if err != nil {
				return newMigrationError(ErrCodeTableData, "序列化迁移记录失败", err)
			}
			batchPayloads = append(batchPayloads, string(payloadJSON))
			insertedCount++

			state.ProcessedCount++
			if strategy == source.StrategyCursor && cursorColumn != "" {
				state.CursorValue = row[cursorColumn]
			}
			if strategy == source.StrategyOffset {
				offset++
			}
		}

		if len(batchPayloads) > 0 {
			if err := retryWithBackoff(3, 100*time.Millisecond, func() error {
				return r.insertRecordBatch(targetTableID, batchPayloads)
			}); err != nil {
				return newMigrationError(ErrCodeTableData, "写入目标记录批次失败", err)
			}
		}

		if state.ProcessedCount%int64(r.cfg.Options.CheckpointInterval) == 0 {
			if err := r.saveTableState(sourceTable, *state); err != nil {
				return err
			}
		}

		zap.L().Info("批次迁移完成",
			zap.String("migration_id", r.migrationID),
			zap.String("table", sourceTable),
			zap.Int64("processed", state.ProcessedCount),
			zap.Int64("inserted", insertedCount),
		)

		if len(rows) < r.cfg.Data.BatchSize {
			return nil
		}
	}
}

func (r *Runner) insertRecordBatch(targetTableID string, payloads []string) error {
	if len(payloads) == 0 {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		records := make([]models.Record, 0, len(payloads))
		for _, payload := range payloads {
			records = append(records, models.Record{
				TableID: targetTableID,
				Data:    payload,
				Version: 1,
			})
		}
		if err := tx.Create(&records).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *Runner) normalizeRow(schema *source.TableSchema, row map[string]interface{}) map[string]interface{} {
	payload := make(map[string]interface{}, len(row))
	for _, column := range schema.Columns {
		fieldType, _ := r.mapper.Map(column.Type)
		payload[column.Name] = normalizeValueForField(fieldType, row[column.Name])
	}
	return payload
}

func normalizeValueForField(fieldType string, value interface{}) interface{} {
	switch fieldType {
	case "boolean":
		switch v := value.(type) {
		case bool:
			return v
		case int64:
			return v != 0
		case float64:
			return v != 0
		case string:
			return v == "1" || strings.EqualFold(v, "true")
		default:
			return value
		}
	case "date", "datetime", "string", "text":
		switch v := value.(type) {
		case []byte:
			return string(v)
		default:
			return value
		}
	case "json":
		switch v := value.(type) {
		case string:
			var parsed interface{}
			if err := json.Unmarshal([]byte(v), &parsed); err == nil {
				return parsed
			}
			return v
		default:
			return value
		}
	default:
		return value
	}
}

func (r *Runner) validationAndReportForExisting(databaseID string, tablePlan PreviewTablePlan) (MigrationTableReport, error) {
	var targetTable models.Table
	if err := r.db.Where("database_id = ? AND name = ? AND deleted_at IS NULL", databaseID, tablePlan.TargetTable).First(&targetTable).Error; err != nil {
		return MigrationTableReport{}, err
	}
	var fieldCount int64
	if err := r.db.Model(&models.Field{}).Where("table_id = ? AND deleted_at IS NULL", targetTable.ID).Count(&fieldCount).Error; err != nil {
		return MigrationTableReport{}, err
	}
	var recordCount int64
	if err := r.db.Raw("SELECT COUNT(*) FROM records WHERE table_id = ? AND deleted_at IS NULL", targetTable.ID).Scan(&recordCount).Error; err != nil {
		return MigrationTableReport{}, err
	}
	return MigrationTableReport{
		Source:          tablePlan.SourceTable,
		Target:          tablePlan.TargetTable,
		Status:          TableStatusCompleted,
		FieldsCreated:   int(fieldCount),
		RecordsInserted: recordCount,
		Warnings:        tablePlan.TypeMappingWarnings,
	}, nil
}

func (r *Runner) validateMigration(databaseID string, tablePlans []PreviewTablePlan) (ValidationReport, error) {
	report := ValidationReport{
		Status:  ValidationPassed,
		Details: make([]ValidationTableDetail, 0, len(tablePlans)),
	}

	for _, tablePlan := range tablePlans {
		var targetTable models.Table
		if err := r.db.Where("database_id = ? AND name = ? AND deleted_at IS NULL", databaseID, tablePlan.TargetTable).First(&targetTable).Error; err != nil {
			return ValidationReport{}, err
		}
		schema, err := r.src.GetTableSchema("", tablePlan.SourceTable)
		if err != nil {
			return ValidationReport{}, err
		}
		tableValidation, err := r.validateTable(targetTable.ID, tablePlan, schema, r.src)
		if err != nil {
			return ValidationReport{}, err
		}
		report.TablesChecked += tableValidation.TablesChecked
		report.TablesPassed += tableValidation.TablesPassed
		report.TablesFailed += tableValidation.TablesFailed
		report.TablesWarnings += tableValidation.TablesWarnings
		report.Details = append(report.Details, tableValidation.Details...)
	}

	switch {
	case report.TablesFailed > 0:
		report.Status = ValidationFailed
	case report.TablesWarnings > 0:
		report.Status = ValidationPassedWithWarn
	default:
		report.Status = ValidationPassed
	}

	return report, nil
}

func (r *Runner) validateTable(targetTableID string, tablePlan PreviewTablePlan, schema *source.TableSchema, src source.Source) (ValidationReport, error) {
	report := ValidationReport{
		Status: ValidationPassed,
		Details: []ValidationTableDetail{{
			Table:          tablePlan.TargetTable,
			StructureMatch: true,
			RowCountMatch:  true,
		}},
		TablesChecked: 1,
	}
	detail := &report.Details[0]

	var targetFields []models.Field
	if err := r.db.Where("table_id = ? AND deleted_at IS NULL", targetTableID).Find(&targetFields).Error; err != nil {
		return ValidationReport{}, err
	}
	sourceFieldNames := make(map[string]struct{}, len(schema.Columns))
	for _, column := range schema.Columns {
		sourceFieldNames[column.Name] = struct{}{}
	}
	if len(targetFields) != len(schema.Columns) {
		detail.StructureMatch = false
		detail.Warnings = append(detail.Warnings, "字段数量不一致")
	}
	for _, field := range targetFields {
		if _, ok := sourceFieldNames[field.Name]; !ok {
			detail.StructureMatch = false
			detail.Warnings = append(detail.Warnings, fmt.Sprintf("目标字段 %q 在源表中不存在", field.Name))
		}
	}

	sourceCount, err := src.EstimateRowCount("", tablePlan.SourceTable)
	if err != nil {
		return ValidationReport{}, err
	}
	var targetCount int64
	if err := r.db.Raw("SELECT COUNT(*) FROM records WHERE table_id = ? AND deleted_at IS NULL", targetTableID).Scan(&targetCount).Error; err != nil {
		return ValidationReport{}, err
	}
	if sourceCount != targetCount {
		detail.RowCountMatch = false
		detail.Warnings = append(detail.Warnings, fmt.Sprintf("行数不一致: source=%d target=%d", sourceCount, targetCount))
	}

	sampleChecked, sampleMismatch, sampleWarnings, err := r.sampleCompare(targetTableID, tablePlan, schema, src)
	if err != nil {
		return ValidationReport{}, err
	}
	detail.SampleChecked = sampleChecked
	detail.SampleMismatch = sampleMismatch
	detail.Warnings = append(detail.Warnings, sampleWarnings...)

	statsWarnings, err := r.compareColumnStats(targetTableID, tablePlan, schema, src)
	if err != nil {
		return ValidationReport{}, err
	}
	detail.Warnings = append(detail.Warnings, statsWarnings...)

	failed := !detail.StructureMatch || !detail.RowCountMatch || detail.SampleMismatch > 0
	switch {
	case failed:
		report.Status = ValidationFailed
		report.TablesFailed = 1
	case len(detail.Warnings) > 0:
		report.Status = ValidationPassedWithWarn
		report.TablesPassed = 1
		report.TablesWarnings = 1
	default:
		report.TablesPassed = 1
	}

	if report.TablesPassed == 0 && report.TablesFailed == 0 {
		report.TablesPassed = 1
	}
	return report, nil
}

func (r *Runner) sampleCompare(targetTableID string, tablePlan PreviewTablePlan, schema *source.TableSchema, src source.Source) (int, int, []string, error) {
	cursorColumn := r.resolveCursorColumn(schema)
	if cursorColumn == "" {
		return 0, 0, []string{fmt.Sprintf("%s 表 %q 缺少游标列，跳过内容抽样校验", ErrCodeOffsetFallbackWarn, tablePlan.TargetTable)}, nil
	}

	strategy := r.pickStrategyWithSource(src, tablePlan.SourceTable, schema)
	sampleSize := computeSampleSize(schema.RowEstimate)
	if sampleSize == 0 {
		return 0, 0, nil, nil
	}

	checked := 0
	mismatch := 0
	warnings := []string{}
	var cursorValue interface{}
	for checked < sampleSize {
		rows, err := src.QueryRows("", tablePlan.SourceTable, source.QueryOptions{
			Strategy:     strategy,
			CursorColumn: cursorColumn,
			CursorValue:  normalizeCursorValue(cursorValue),
			Limit:        int64(minInt(r.cfg.Data.BatchSize, sampleSize-checked)),
		})
		if err != nil {
			return 0, 0, nil, err
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			sourcePayload := r.normalizeRow(schema, row)
			targetPayload, err := r.fetchTargetPayloadByCursor(targetTableID, cursorColumn, row[cursorColumn])
			if err != nil {
				return 0, 0, nil, err
			}
			if targetPayload == nil {
				mismatch++
				warnings = append(warnings, fmt.Sprintf("缺少目标记录: %s=%v", cursorColumn, row[cursorColumn]))
			} else {
				diffField := firstDifferentField(sourcePayload, targetPayload)
				if diffField != "" {
					mismatch++
					warnings = append(warnings, fmt.Sprintf("字段 %q 内容不一致: %s=%v", diffField, cursorColumn, row[cursorColumn]))
				}
			}
			checked++
			cursorValue = row[cursorColumn]
			if checked >= sampleSize {
				break
			}
		}
		if len(rows) < r.cfg.Data.BatchSize {
			break
		}
	}
	return checked, mismatch, warnings, nil
}

func (r *Runner) compareColumnStats(targetTableID string, tablePlan PreviewTablePlan, schema *source.TableSchema, src source.Source) ([]string, error) {
	targetRecords, err := r.loadTargetRecords(targetTableID)
	if err != nil {
		return nil, err
	}
	sourceRows, err := r.loadAllSourceRows(tablePlan.SourceTable, schema, src)
	if err != nil {
		return nil, err
	}

	warnings := []string{}
	for _, column := range schema.Columns {
		fieldType, _ := r.mapper.Map(column.Type)
		switch fieldType {
		case "number":
			sourceSum, sourceCount := sumNumericFieldFromMaps(sourceRows, column.Name)
			targetSum, targetCount := sumNumericFieldFromMaps(targetRecords, column.Name)
			if sourceCount != targetCount || math.Abs(sourceSum-targetSum) > 1e-9 {
				warnings = append(warnings, fmt.Sprintf("数值统计不一致: %s source(sum=%.4f,count=%d) target(sum=%.4f,count=%d)", column.Name, sourceSum, sourceCount, targetSum, targetCount))
			}
		case "date", "datetime":
			sourceMin, sourceMax := minMaxStringFieldFromMaps(sourceRows, column.Name)
			targetMin, targetMax := minMaxStringFieldFromMaps(targetRecords, column.Name)
			if sourceMin != targetMin || sourceMax != targetMax {
				warnings = append(warnings, fmt.Sprintf("日期范围不一致: %s source(%s~%s) target(%s~%s)", column.Name, sourceMin, sourceMax, targetMin, targetMax))
			}
		}
	}
	return warnings, nil
}

func (r *Runner) loadTargetRecords(targetTableID string) ([]map[string]interface{}, error) {
	var records []models.Record
	if err := r.db.Where("table_id = ? AND deleted_at IS NULL", targetTableID).Find(&records).Error; err != nil {
		return nil, err
	}
	result := make([]map[string]interface{}, 0, len(records))
	for _, record := range records {
		payload := map[string]interface{}{}
		if err := json.Unmarshal([]byte(record.Data), &payload); err != nil {
			return nil, err
		}
		result = append(result, payload)
	}
	return result, nil
}

func (r *Runner) loadAllSourceRows(sourceTable string, schema *source.TableSchema, src source.Source) ([]map[string]interface{}, error) {
	strategy := r.pickStrategyWithSource(src, sourceTable, schema)
	cursorColumn := r.resolveCursorColumn(schema)
	var cursorValue interface{}
	offset := int64(0)
	result := []map[string]interface{}{}
	for {
		rows, err := src.QueryRows("", sourceTable, source.QueryOptions{
			Strategy:     strategy,
			CursorColumn: cursorColumn,
			CursorValue:  normalizeCursorValue(cursorValue),
			Offset:       offset,
			Limit:        int64(r.cfg.Data.BatchSize),
		})
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return result, nil
		}
		for _, row := range rows {
			result = append(result, r.normalizeRow(schema, row))
			if strategy == source.StrategyCursor && cursorColumn != "" {
				cursorValue = row[cursorColumn]
			} else {
				offset++
			}
		}
		if len(rows) < r.cfg.Data.BatchSize {
			return result, nil
		}
	}
}

func (r *Runner) fetchTargetPayloadByCursor(targetTableID, cursorColumn string, cursorValue interface{}) (map[string]interface{}, error) {
	query := r.db.Model(&models.Record{}).Select("data").Where("table_id = ? AND deleted_at IS NULL", targetTableID)
	switch r.db.Name() {
	case "sqlite":
		query = query.Where("JSON_EXTRACT(data, ?) = ?", "$."+cursorColumn, cursorValue)
	default:
		payload, err := json.Marshal(map[string]interface{}{cursorColumn: cursorValue})
		if err != nil {
			return nil, err
		}
		query = query.Where("data @> ?", string(payload))
	}

	var raw string
	if err := query.Limit(1).Scan(&raw).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		if raw == "" {
			return nil, nil
		}
		return nil, err
	}
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	payload := map[string]interface{}{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (r *Runner) finalizeValidation(tableReports []MigrationTableReport) ValidationReport {
	report := ValidationReport{
		Status:  ValidationPassed,
		Details: make([]ValidationTableDetail, 0, len(tableReports)),
	}
	for _, table := range tableReports {
		report.TablesChecked++
		detail := ValidationTableDetail{
			Table:          table.Target,
			StructureMatch: true,
			RowCountMatch:  table.Error == "",
		}
		if table.Error != "" {
			report.TablesFailed++
			report.Status = ValidationFailed
			detail.StructureMatch = false
			detail.Warnings = append(detail.Warnings, table.Error)
		} else {
			report.TablesPassed++
			if len(table.Warnings) > 0 && report.Status == ValidationPassed {
				report.Status = ValidationPassedWithWarn
			}
			if len(table.Warnings) > 0 {
				report.TablesWarnings++
				detail.Warnings = append(detail.Warnings, table.Warnings...)
			}
		}
		report.Details = append(report.Details, detail)
	}
	return report
}

func (r *Runner) rollbackTable(tableID string) error {
	now := time.Now()
	if err := r.db.Model(&models.Record{}).Where("table_id = ? AND deleted_at IS NULL", tableID).Update("deleted_at", now).Error; err != nil {
		return err
	}
	if err := r.db.Model(&models.Field{}).Where("table_id = ? AND deleted_at IS NULL", tableID).Update("deleted_at", now).Error; err != nil {
		return err
	}
	if err := r.db.Model(&models.Table{}).Where("id = ? AND deleted_at IS NULL", tableID).Update("deleted_at", now).Error; err != nil {
		return err
	}
	return nil
}

func (r *Runner) loadOrInitState(plan *compiledPlan) (MigrationState, error) {
	if r.opts.ResumeID != "" {
		state, err := r.store.Load(r.opts.ResumeID)
		if err != nil {
			return MigrationState{}, err
		}
		if state.Tables == nil {
			state.Tables = map[string]TableState{}
		}
		if state.MigrationID == "" {
			state.MigrationID = r.migrationID
		}
		r.stateMu.Lock()
		r.state = state
		r.stateMu.Unlock()
		return state, nil
	}

	state := MigrationState{
		MigrationID: r.migrationID,
		Source:      normalizeLower(r.cfg.Source.Type) + ":" + r.cfg.BuildSourceDSN(),
		TargetDB:    plan.TargetDatabase,
		StartedAt:   time.Now().UTC(),
		Tables:      map[string]TableState{},
	}
	for _, table := range plan.Tables {
		state.Tables[table.SourceTable] = TableState{
			Status:        TableStatusPending,
			TotalEstimate: table.EstimatedRows,
		}
	}
	if err := r.store.Save(state); err != nil {
		return MigrationState{}, err
	}
	r.stateMu.Lock()
	r.state = state
	r.stateMu.Unlock()
	return state, nil
}

func (r *Runner) pickStrategy(tableName string, schema *source.TableSchema) source.PaginationStrategy {
	return r.pickStrategyWithSource(r.src, tableName, schema)
}

func (r *Runner) pickStrategyWithSource(src source.Source, tableName string, schema *source.TableSchema) source.PaginationStrategy {
	if r.cfg.Data.PaginationStrategy == PaginationOffset {
		return source.StrategyOffset
	}
	if strings.TrimSpace(r.cfg.Data.CursorColumn) != "" {
		return source.StrategyCursor
	}
	if src == nil {
		if len(schema.PrimaryKey) == 1 {
			return source.StrategyCursor
		}
		for _, key := range schema.UniqueKeys {
			if len(key) == 1 {
				return source.StrategyCursor
			}
		}
		return source.StrategyOffset
	}
	return src.RecommendPaginationStrategy("", tableName)
}

func (r *Runner) resolveCursorColumn(schema *source.TableSchema) string {
	if strings.TrimSpace(r.cfg.Data.CursorColumn) != "" {
		return r.cfg.Data.CursorColumn
	}
	if len(schema.PrimaryKey) == 1 {
		return schema.PrimaryKey[0]
	}
	for _, uniqueKey := range schema.UniqueKeys {
		if len(uniqueKey) == 1 {
			return uniqueKey[0]
		}
	}
	return ""
}

func (r *Runner) targetTableName(sourceTable string) string {
	if renamed, ok := r.cfg.Tables.Rename[sourceTable]; ok && strings.TrimSpace(renamed) != "" {
		return renamed
	}
	return sourceTable
}

func (r *Runner) sourceDatabaseName() string {
	if strings.TrimSpace(r.cfg.Source.Database) != "" {
		return r.cfg.Source.Database
	}
	if normalizeLower(r.cfg.Source.Type) == "sqlite" {
		return r.cfg.EffectiveTargetDatabase()
	}
	return ""
}

func toInt64(value interface{}) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case json.Number:
		i, _ := v.Int64()
		return i
	case string:
		i, _ := strconv.ParseInt(v, 10, 64)
		return i
	default:
		return 0
	}
}

func normalizeCursorValue(value interface{}) interface{} {
	switch v := value.(type) {
	case float64:
		if v == float64(int64(v)) {
			return int64(v)
		}
		return v
	default:
		return value
	}
}

func (r *Runner) recordExists(tableID, fieldName string, fieldValue interface{}) (bool, error) {
	query := r.db.Model(&models.Record{}).Where("table_id = ? AND deleted_at IS NULL", tableID)
	switch r.db.Name() {
	case "sqlite":
		query = query.Where("JSON_EXTRACT(data, ?) = ?", "$."+fieldName, fieldValue)
	default:
		payload, err := json.Marshal(map[string]interface{}{fieldName: fieldValue})
		if err != nil {
			return false, err
		}
		query = query.Where("data @> ?", string(payload))
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Runner) ensureSource() error {
	if r.src != nil {
		return nil
	}
	src, err := r.sourceFactory()
	if err != nil {
		return err
	}
	if err := src.Connect(r.cfg.BuildSourceDSN()); err != nil {
		return newMigrationError(ErrCodeSourceConnect, "连接源数据库失败", err)
	}
	r.src = src
	return nil
}

func (r *Runner) closeSource() {
	if r.src == nil {
		return
	}
	_ = r.src.Close()
	r.src = nil
}

func (r *Runner) getTableState(tableName string) TableState {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	return r.state.Tables[tableName]
}

func (r *Runner) saveTableState(tableName string, tableState TableState) error {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	if r.state.Tables == nil {
		r.state.Tables = map[string]TableState{}
	}
	r.state.Tables[tableName] = tableState
	return r.store.Save(r.state)
}

func (r *Runner) executeTablePlans(databaseID string, plans []PreviewTablePlan) []tableExecutionResult {
	workerCount := r.cfg.Data.MaxConcurrentTables
	if !r.cfg.Options.ContinueOnError && workerCount > 1 {
		workerCount = 1
	}
	if workerCount <= 1 {
		results := make([]tableExecutionResult, 0, len(plans))
		for _, plan := range plans {
			tableState := r.getTableState(plan.SourceTable)
			if tableState.Status == TableStatusCompleted {
				report, err := r.validationAndReportForExisting(databaseID, plan)
				results = append(results, tableExecutionResult{report: report, err: err})
				continue
			}
			report, err := r.runTable(databaseID, plan)
			results = append(results, tableExecutionResult{report: report, err: err})
			if err != nil && !r.cfg.Options.ContinueOnError {
				break
			}
		}
		return results
	}

	jobs := make(chan PreviewTablePlan)
	resultsCh := make(chan tableExecutionResult, len(plans))
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for plan := range jobs {
				tableState := r.getTableState(plan.SourceTable)
				if tableState.Status == TableStatusCompleted {
					report, err := r.validationAndReportForExisting(databaseID, plan)
					resultsCh <- tableExecutionResult{report: report, err: err}
					continue
				}
				report, err := r.runTable(databaseID, plan)
				resultsCh <- tableExecutionResult{report: report, err: err}
			}
		}()
	}

	go func() {
		for _, plan := range plans {
			jobs <- plan
		}
		close(jobs)
		wg.Wait()
		close(resultsCh)
	}()

	results := make([]tableExecutionResult, 0, len(plans))
	for result := range resultsCh {
		results = append(results, result)
	}
	return results
}

func retryWithBackoff(attempts int, initialDelay time.Duration, fn func() error) error {
	if attempts <= 0 {
		attempts = 1
	}
	delay := initialDelay
	if delay <= 0 {
		delay = 50 * time.Millisecond
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := fn(); err != nil {
			lastErr = err
			if attempt == attempts {
				break
			}
			zap.L().Warn("批次插入失败，准备重试",
				zap.Int("attempt", attempt),
				zap.Duration("delay", delay),
				zap.Error(err),
			)
			time.Sleep(delay)
			delay *= 2
			continue
		}
		return nil
	}
	return lastErr
}

func firstDifferentField(sourcePayload, targetPayload map[string]interface{}) string {
	for key, sourceValue := range sourcePayload {
		targetValue, ok := targetPayload[key]
		if !ok {
			return key
		}
		if !jsonLikeEqual(sourceValue, targetValue) {
			return key
		}
	}
	return ""
}

func jsonLikeEqual(left, right interface{}) bool {
	leftJSON, err := json.Marshal(left)
	if err != nil {
		return false
	}
	rightJSON, err := json.Marshal(right)
	if err != nil {
		return false
	}
	return string(leftJSON) == string(rightJSON)
}

func computeSampleSize(total int64) int {
	if total <= 0 {
		return 0
	}
	if total <= 20 {
		return int(total)
	}
	size := int(total / 20)
	if size < 1 {
		size = 1
	}
	if size > 10 {
		size = 10
	}
	return size
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func sumNumericFieldFromMaps(rows []map[string]interface{}, fieldName string) (float64, int) {
	sum := 0.0
	count := 0
	for _, row := range rows {
		value, ok := row[fieldName]
		if !ok {
			continue
		}
		number, ok := toFloat64(value)
		if !ok {
			continue
		}
		sum += number
		count++
	}
	return sum, count
}

func minMaxStringFieldFromMaps(rows []map[string]interface{}, fieldName string) (string, string) {
	var minValue string
	var maxValue string
	for _, row := range rows {
		value, ok := row[fieldName].(string)
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		if minValue == "" || value < minValue {
			minValue = value
		}
		if maxValue == "" || value > maxValue {
			maxValue = value
		}
	}
	return minValue, maxValue
}

func toFloat64(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	case json.Number:
		number, err := v.Float64()
		return number, err == nil
	case string:
		number, err := strconv.ParseFloat(v, 64)
		return number, err == nil
	default:
		return 0, false
	}
}
