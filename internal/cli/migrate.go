package cli

import (
	"fmt"
	"time"

	"github.com/jiangfire/cornerstone/internal/config"
	"github.com/jiangfire/cornerstone/internal/db"
	applog "github.com/jiangfire/cornerstone/pkg/log"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "run database migrations",
	Long:  `Run all database schema migrations, including creating missing tables and indexes.`,
	RunE:  runMigrate,
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}

func runMigrate(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := applog.InitLogger(cfg.Logger); err != nil {
		return fmt.Errorf("failed to init logger: %w", err)
	}

	if err := retryOperation(func() error {
		return db.InitDB(cfg.Database)
	}, 3, time.Second); err != nil {
		return fmt.Errorf("failed to init database: %w", err)
	}
	defer func() { _ = db.CloseDB() }()

	if err := db.Migrate(); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	fmt.Println("database migration completed")
	return nil
}
