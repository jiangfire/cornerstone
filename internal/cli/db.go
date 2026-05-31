package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/jiangfire/cornerstone/internal/config"
	appdb "github.com/jiangfire/cornerstone/internal/db"
	"github.com/jiangfire/cornerstone/internal/services"
	"github.com/jiangfire/cornerstone/pkg/db"
	applog "github.com/jiangfire/cornerstone/pkg/log"
	"github.com/spf13/cobra"
)

func ensureDB() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}
	if err := applog.InitLogger(cfg.Logger); err != nil {
		return fmt.Errorf("初始化日志失败: %w", err)
	}
	if err := retryOperation(func() error {
		return appdb.InitDB(cfg.Database)
	}, 3, time.Second); err != nil {
		return fmt.Errorf("初始化数据库失败: %w", err)
	}
	if err := appdb.Migrate(); err != nil {
		return fmt.Errorf("迁移失败: %w", err)
	}
	return nil
}

func getMasterTokenID() (string, error) {
	masterToken := os.Getenv("MASTER_TOKEN")
	if masterToken == "" {
		return "", fmt.Errorf("请设置 MASTER_TOKEN 环境变量")
	}
	return masterToken, nil
}

func printJSON(v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "数据库管理",
	Long:  `管理 Cornerstone 数据库资源。支持 list、create、get、update、delete 子命令。`,
}

var dbListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出所有数据库",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureDB(); err != nil {
			return err
		}
		defer func() { _ = appdb.CloseDB() }()

		token, err := getMasterTokenID()
		if err != nil {
			return err
		}
		svc := services.NewDatabaseService(db.DB())
		databases, err := svc.ListDatabases(token)
		if err != nil {
			return err
		}
		return printJSON(databases)
	},
}

var dbCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "创建数据库",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureDB(); err != nil {
			return err
		}
		defer func() { _ = appdb.CloseDB() }()

		desc, _ := cmd.Flags().GetString("description")
		token, err := getMasterTokenID()
		if err != nil {
			return err
		}
		svc := services.NewDatabaseService(db.DB())
		database, err := svc.CreateDatabase(services.CreateDBRequest{
			Name:        args[0],
			Description: desc,
		}, token)
		if err != nil {
			return err
		}
		return printJSON(database)
	},
}

var dbGetCmd = &cobra.Command{
	Use:   "get [id]",
	Short: "获取数据库详情",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureDB(); err != nil {
			return err
		}
		defer func() { _ = appdb.CloseDB() }()

		token, err := getMasterTokenID()
		if err != nil {
			return err
		}
		svc := services.NewDatabaseService(db.DB())
		database, err := svc.GetDatabase(args[0], token)
		if err != nil {
			return err
		}
		return printJSON(database)
	},
}

var dbUpdateCmd = &cobra.Command{
	Use:   "update [id]",
	Short: "更新数据库",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureDB(); err != nil {
			return err
		}
		defer func() { _ = appdb.CloseDB() }()

		name, _ := cmd.Flags().GetString("name")
		desc, _ := cmd.Flags().GetString("description")
		token, err := getMasterTokenID()
		if err != nil {
			return err
		}
		svc := services.NewDatabaseService(db.DB())
		database, err := svc.UpdateDatabase(args[0], services.UpdateDBRequest{
			Name:        name,
			Description: desc,
		}, token)
		if err != nil {
			return err
		}
		return printJSON(database)
	},
}

var dbDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "删除数据库",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureDB(); err != nil {
			return err
		}
		defer func() { _ = appdb.CloseDB() }()

		token, err := getMasterTokenID()
		if err != nil {
			return err
		}
		svc := services.NewDatabaseService(db.DB())
		if err := svc.DeleteDatabase(args[0], token); err != nil {
			return err
		}
		fmt.Println("数据库已删除")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(dbCmd)
	dbCmd.AddCommand(dbListCmd)
	dbCmd.AddCommand(dbCreateCmd)
	dbCmd.AddCommand(dbGetCmd)
	dbCmd.AddCommand(dbUpdateCmd)
	dbCmd.AddCommand(dbDeleteCmd)

	dbCreateCmd.Flags().StringP("description", "d", "", "数据库描述")
	dbUpdateCmd.Flags().StringP("name", "n", "", "新名称")
	dbUpdateCmd.Flags().StringP("description", "d", "", "新描述")
}
