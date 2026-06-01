package migration

import "time"

const (
	StatusCompleted           = "completed"
	StatusCompletedWithIssues = "completed_with_issues"
	StatusFailed              = "failed"

	ValidationPassed         = "passed"
	ValidationPassedWithWarn = "passed_with_warnings"
	ValidationFailed         = "failed"
)

type PreviewPlan struct {
	Source             PreviewSource      `json:"source"`
	TargetDatabase     string             `json:"target_database"`
	Tables             []PreviewTablePlan `json:"tables"`
	TotalEstimatedRows int64              `json:"total_estimated_rows"`
}

type PreviewSource struct {
	Type     string `json:"type"`
	Database string `json:"database"`
}

type PreviewTablePlan struct {
	SourceTable         string   `json:"source_table"`
	TargetTable         string   `json:"target_table"`
	Fields              int      `json:"fields"`
	EstimatedRows       int64    `json:"estimated_rows"`
	TypeMappingWarnings []string `json:"type_mapping_warnings,omitempty"`
	MigrationStrategy   string   `json:"migration_strategy"`
}

type MigrationReport struct {
	MigrationID string                 `json:"migration_id"`
	Status     string                 `json:"status"`
	StartedAt  time.Time              `json:"started_at"`
	FinishedAt time.Time              `json:"finished_at"`
	Summary    ReportSummary          `json:"summary"`
	Tables     []MigrationTableReport `json:"tables"`
	Validation ValidationReport       `json:"validation"`
}

type ReportSummary struct {
	TablesTotal     int   `json:"tables_total"`
	TablesSuccess   int   `json:"tables_success"`
	TablesFailed    int   `json:"tables_failed"`
	RecordsTotal    int64 `json:"records_total"`
	RecordsInserted int64 `json:"records_inserted"`
}

type MigrationTableReport struct {
	Source          string   `json:"source"`
	Target          string   `json:"target"`
	Status          string   `json:"status"`
	FieldsCreated   int      `json:"fields_created"`
	RecordsInserted int64    `json:"records_inserted"`
	Error           string   `json:"error,omitempty"`
	Warnings        []string `json:"warnings,omitempty"`
}

type ValidationReport struct {
	Status         string                  `json:"status"`
	TablesChecked  int                     `json:"tables_checked"`
	TablesPassed   int                     `json:"tables_passed"`
	TablesFailed   int                     `json:"tables_failed"`
	TablesWarnings int                     `json:"tables_warnings"`
	Details        []ValidationTableDetail `json:"details,omitempty"`
}

type ValidationTableDetail struct {
	Table          string   `json:"table"`
	StructureMatch bool     `json:"structure_match"`
	RowCountMatch  bool     `json:"row_count_match"`
	SampleChecked  int      `json:"sample_checked,omitempty"`
	SampleMismatch int      `json:"sample_mismatch,omitempty"`
	Warnings       []string `json:"warnings,omitempty"`
}
