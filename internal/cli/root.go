package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var Version = "dev"

// jsonOutput controls whether all CLI output uses structured JSON.
var jsonOutput bool

// tokenOverride allows passing auth token via --token flag instead of MASTER_TOKEN env var.
var tokenOverride string

// CLI exit codes for machine-readable error classification.
const (
	ExitSuccess         = 0
	ExitGeneralError    = 1
	ExitValidationError = 2
	ExitNotFound        = 3
	ExitPermission      = 4
	ExitServerError     = 5
)

var rootCmd = &cobra.Command{
	Use:   "cornerstone",
	Short: "Cornerstone - lightweight data asset platform CLI",
	Long: `Cornerstone is a lightweight data asset platform for testing, development, and internal data management.
Core positioning: "Database + Token API + Query DSL + AI Assistant + MCP Protocol".

You can manage data assets directly via CLI, or start the HTTP API + MCP server for AI Agent integration.`,
	Version: Version,
}

func Execute() {
	rootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return &cliError{code: ExitValidationError, message: err.Error()}
	})

	err := rootCmd.Execute()
	if err == nil {
		return
	}

	code := classifyExitCode(err)

	if jsonOutput {
		out, _ := json.Marshal(map[string]interface{}{
			"ok":    false,
			"error": map[string]interface{}{"code": exitCodeName(code), "message": err.Error()},
		})
		fmt.Fprintln(os.Stderr, string(out))
	} else {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(code)
}

// cliError carries a semantic exit code alongside an error message.
type cliError struct {
	code    int
	message string
}

func (e *cliError) Error() string { return e.message }

// classifyExitCode maps an error to a semantic exit code by inspecting the error message.
// Service layer errors are in English; we match known patterns.
func classifyExitCode(err error) int {
	if cliErr, ok := err.(*cliError); ok {
		return cliErr.code
	}

	msg := err.Error()

	// Permission denied
	if strings.Contains(msg, "permission denied") || strings.Contains(msg, "unauthorized") {
		return ExitPermission
	}

	// Not found
	if strings.Contains(msg, "not found") || strings.Contains(msg, "does not exist") {
		return ExitNotFound
	}

	// Validation errors (CLI-side)
	if strings.Contains(msg, "invalid") || strings.Contains(msg, "required") || strings.Contains(msg, "format") {
		return ExitValidationError
	}

	// Server/infra errors
	if strings.Contains(msg, "config") || strings.Contains(msg, "init") || strings.Contains(msg, "database") {
		return ExitServerError
	}

	return ExitGeneralError
}

func exitCodeName(code int) string {
	switch code {
	case ExitSuccess:
		return "SUCCESS"
	case ExitValidationError:
		return "VALIDATION_ERROR"
	case ExitNotFound:
		return "NOT_FOUND"
	case ExitPermission:
		return "PERMISSION_DENIED"
	case ExitServerError:
		return "SERVER_ERROR"
	default:
		return "UNKNOWN_ERROR"
	}
}

func init() {
	rootCmd.SetVersionTemplate(fmt.Sprintf("Cornerstone %s\n", Version))
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output all results as structured JSON (machine-readable)")
	rootCmd.PersistentFlags().StringVarP(&tokenOverride, "token", "t", "", "Auth token (alternative to MASTER_TOKEN env var)")
}
