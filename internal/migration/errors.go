package migration

import "fmt"

const (
	ErrCodeSourceConnect      = "MIG-001"
	ErrCodeTargetCreate       = "MIG-002"
	ErrCodeTableSchema        = "MIG-003"
	ErrCodeTableData          = "MIG-004"
	ErrCodeTypeMappingWarn    = "MIG-005"
	ErrCodeUnsupportedSource  = "MIG-006"
	ErrCodeCorruptState       = "MIG-007"
	ErrCodeValidationFailed   = "MIG-008"
	ErrCodeOffsetFallbackWarn = "MIG-009"
	ErrCodeConfigInvalid      = "MIG-010"
)

type MigrationError struct {
	Code    string
	Message string
	Cause   error
}

func (e *MigrationError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause == nil {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
}

func (e *MigrationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func newMigrationError(code, message string, cause error) error {
	return &MigrationError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}
