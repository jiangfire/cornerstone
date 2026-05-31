package migration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	TableStatusPending    = "pending"
	TableStatusInProgress = "in_progress"
	TableStatusCompleted  = "completed"
	TableStatusFailed     = "failed"
)

type MigrationState struct {
	MigrationID string                `json:"migration_id"`
	Source      string                `json:"source"`
	TargetDB    string                `json:"target_db"`
	StartedAt   time.Time             `json:"started_at,omitempty"`
	UpdatedAt   time.Time             `json:"updated_at,omitempty"`
	Tables      map[string]TableState `json:"tables"`
}

type TableState struct {
	Status         string      `json:"status"`
	CursorColumn   string      `json:"cursor_column,omitempty"`
	CursorValue    interface{} `json:"cursor_value,omitempty"`
	ProcessedCount int64       `json:"processed_count"`
	TotalEstimate  int64       `json:"total_estimate"`
}

type StateStore struct {
	dir string
}

func NewStateStore(dir string) *StateStore {
	if dir == "" {
		dir = defaultStateDir()
	}
	return &StateStore{dir: dir}
}

func (s *StateStore) Save(state MigrationState) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("创建状态目录失败: %w", err)
	}
	state.UpdatedAt = time.Now().UTC()
	if state.StartedAt.IsZero() {
		state.StartedAt = state.UpdatedAt
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化状态失败: %w", err)
	}
	if err := os.WriteFile(s.pathFor(state.MigrationID), data, 0o600); err != nil {
		return fmt.Errorf("写入状态文件失败: %w", err)
	}
	return nil
}

func (s *StateStore) Load(migrationID string) (MigrationState, error) {
	data, err := os.ReadFile(s.pathFor(migrationID))
	if err != nil {
		return MigrationState{}, fmt.Errorf("读取状态文件失败: %w", err)
	}
	var state MigrationState
	if err := json.Unmarshal(data, &state); err != nil {
		return MigrationState{}, newMigrationError(ErrCodeCorruptState, "解析状态文件失败", err)
	}
	if state.Tables == nil {
		state.Tables = map[string]TableState{}
	}
	return state, nil
}

func (s *StateStore) pathFor(migrationID string) string {
	return filepath.Join(s.dir, migrationID+".state.json")
}

func defaultStateDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".cornerstone/migrations"
	}
	return filepath.Join(home, ".cornerstone", "migrations")
}
