package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/jiangfire/cornerstone/internal/config"
	"github.com/jiangfire/cornerstone/internal/db"
	applog "github.com/jiangfire/cornerstone/pkg/log"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "执行数据库迁移",
	Long:  `执行所有数据库表结构迁移，包括创建缺失的表和索引。`,
	RunE:  runMigrate,
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}

func runMigrate(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	if err := applog.InitLogger(cfg.Logger); err != nil {
		return fmt.Errorf("初始化日志失败: %w", err)
	}

	if err := retryOperation(func() error {
		return db.InitDB(cfg.Database)
	}, 3, time.Second); err != nil {
		return fmt.Errorf("初始化数据库失败: %w", err)
	}
	defer db.CloseDB()

	if err := db.Migrate(); err != nil {
		return fmt.Errorf("迁移失败: %w", err)
	}

	fmt.Println("数据库迁移完成")
	return nil
}
