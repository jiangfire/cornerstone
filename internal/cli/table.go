package cli

import (
	"fmt"

	appdb "github.com/jiangfire/cornerstone/internal/db"
	"github.com/jiangfire/cornerstone/internal/services"
	"github.com/jiangfire/cornerstone/pkg/db"
	"github.com/spf13/cobra"
)

var tableCmd = &cobra.Command{
	Use:   "table",
	Short: "表管理",
	Long:  `管理 Cornerstone 表资源。支持 list、create、get、update、delete 子命令。`,
}

var tableListCmd = &cobra.Command{
	Use:   "list [database-id]",
	Short: "列出数据库下的所有表",
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
		svc := services.NewTableService(db.DB())
		tables, err := svc.ListTables(args[0], token)
		if err != nil {
			return err
		}
		return printJSON(tables)
	},
}

var tableCreateCmd = &cobra.Command{
	Use:   "create [database-id] [name]",
	Short: "在数据库中创建表",
	Args:  cobra.ExactArgs(2),
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
		svc := services.NewTableService(db.DB())
		table, err := svc.CreateTable(services.CreateTableRequest{
			DatabaseID:  args[0],
			Name:        args[1],
			Description: desc,
		}, token)
		if err != nil {
			return err
		}
		return printJSON(table)
	},
}

var tableGetCmd = &cobra.Command{
	Use:   "get [id]",
	Short: "获取表详情",
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
		svc := services.NewTableService(db.DB())
		table, err := svc.GetTable(args[0], token)
		if err != nil {
			return err
		}
		return printJSON(table)
	},
}

var tableUpdateCmd = &cobra.Command{
	Use:   "update [id]",
	Short: "更新表",
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
		svc := services.NewTableService(db.DB())
		table, err := svc.UpdateTable(args[0], services.UpdateTableRequest{
			Name:        name,
			Description: desc,
		}, token)
		if err != nil {
			return err
		}
		return printJSON(table)
	},
}

var tableDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "删除表",
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
		svc := services.NewTableService(db.DB())
		if err := svc.DeleteTable(args[0], token); err != nil {
			return err
		}
		fmt.Println("表已删除")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(tableCmd)
	tableCmd.AddCommand(tableListCmd)
	tableCmd.AddCommand(tableCreateCmd)
	tableCmd.AddCommand(tableGetCmd)
	tableCmd.AddCommand(tableUpdateCmd)
	tableCmd.AddCommand(tableDeleteCmd)

	tableCreateCmd.Flags().StringP("description", "d", "", "表描述")
	tableUpdateCmd.Flags().StringP("name", "n", "", "新名称")
	tableUpdateCmd.Flags().StringP("description", "d", "", "新描述")
}
